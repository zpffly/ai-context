package aictx

import (
	"fmt"
	"sort"
)

var relatedGraphFields = []struct {
	field      string
	targetType string
}{
	{"related_actions", "rpc_mapping"},
	{"related_flows", "flow"},
	{"related_integrations", "integration"},
	{"related_runbooks", "runbook"},
	{"related_capabilities", "capability"},
	{"related_decisions", "decision"},
	{"related_terms", "term"},
}

type graphBuilder struct {
	nodes map[string]GraphNode
	edges map[string]GraphEdge
}

func buildContextGraph(repoRoot string, docs []KnowledgeDoc, rpcIndex map[string]RPCDefinition) ContextGraph {
	builder := graphBuilder{
		nodes: map[string]GraphNode{},
		edges: map[string]GraphEdge{},
	}
	docIDs := map[string]struct{}{}

	sortedDocs := append([]KnowledgeDoc{}, docs...)
	sort.Slice(sortedDocs, func(i, j int) bool {
		return graphDocNodeID(sortedDocs[i]) < graphDocNodeID(sortedDocs[j])
	})
	for _, doc := range sortedDocs {
		id := graphDocNodeID(doc)
		docIDs[id] = struct{}{}
		builder.addNode(GraphNode{
			ID:    id,
			Type:  doc.Type,
			Label: firstNonEmpty(doc.Title, doc.ID),
			Path:  relOrSame(repoRoot, doc.Path),
		})
	}

	for _, rpc := range sortedRPCNames(rpcIndex) {
		def := rpcIndex[rpc]
		serviceID := graphNodeID("thrift_service", def.Service)
		rpcID := graphNodeID("rpc", rpc)
		filePath := relOrSame(repoRoot, def.ThriftFile)
		fileID := graphNodeID("thrift_file", filePath)
		builder.addNode(GraphNode{ID: serviceID, Type: "thrift_service", Label: def.Service})
		builder.addNode(GraphNode{ID: rpcID, Type: "rpc", Label: rpc, Path: filePath})
		builder.addNode(GraphNode{ID: fileID, Type: "thrift_file", Label: filePath, Path: filePath})
		builder.addEdge(GraphEdge{Source: serviceID, Target: rpcID, Type: "defines_rpc"})
		builder.addEdge(GraphEdge{Source: rpcID, Target: fileID, Type: "defined_in"})
	}

	for _, doc := range sortedDocs {
		sourceID := graphDocNodeID(doc)
		for _, related := range relatedGraphFields {
			for _, ref := range collectIDRefs(doc.Raw, related.field) {
				builder.addEdge(GraphEdge{
					Source: sourceID,
					Target: graphNodeID(related.targetType, ref),
					Type:   related.field,
				})
			}
		}
		for _, service := range collectIDRefs(doc.Raw, "owning_services") {
			targetID := graphNodeID("service", service)
			builder.addNode(GraphNode{ID: targetID, Type: "service", Label: service})
			builder.addEdge(GraphEdge{Source: sourceID, Target: targetID, Type: "owned_by_service"})
		}
		addRPCEdges(builder, sourceID, doc.Raw, "primary_rpc")
		addRPCEdges(builder, sourceID, doc.Raw, "primary_rpcs")
		addRPCEdges(builder, sourceID, doc.Raw, "request_entry")
		addRPCEdges(builder, sourceID, doc.Raw, "callback_entry")
		addAvoidRPCEdges(builder, sourceID, doc.Raw["avoid_rpc"])
		addAvoidRPCEdges(builder, sourceID, doc.Raw["avoid_rpcs"])
		avoidSet := avoidRPCSet(doc.Raw)
		for _, rpc := range doc.RPCs {
			if _, ok := avoidSet[rpc]; ok {
				continue
			}
			builder.addEdge(GraphEdge{Source: sourceID, Target: graphNodeID("rpc", rpc), Type: "references_rpc"})
		}
		if doc.Type == "flow" {
			builder.addFlowSteps(repoRoot, doc, docIDs)
		}
	}

	return ContextGraph{
		Version: 1,
		Kind:    "ai-context-graph",
		Nodes:   builder.sortedNodes(),
		Edges:   builder.sortedEdges(),
	}
}

func (b graphBuilder) addNode(node GraphNode) {
	if _, ok := b.nodes[node.ID]; ok {
		return
	}
	b.nodes[node.ID] = node
}

func (b graphBuilder) addEdge(edge GraphEdge) {
	if edge.Source == "" || edge.Target == "" || edge.Type == "" {
		return
	}
	key := edge.Source + "\x00" + edge.Type + "\x00" + edge.Target + "\x00" + edge.Reason
	if _, ok := b.edges[key]; ok {
		return
	}
	b.edges[key] = edge
}

func (b graphBuilder) sortedNodes() []GraphNode {
	nodes := make([]GraphNode, 0, len(b.nodes))
	for _, node := range b.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	return nodes
}

func (b graphBuilder) sortedEdges() []GraphEdge {
	edges := make([]GraphEdge, 0, len(b.edges))
	for _, edge := range b.edges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Type != edges[j].Type {
			return edges[i].Type < edges[j].Type
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		return edges[i].Reason < edges[j].Reason
	})
	return edges
}

func (b graphBuilder) addFlowSteps(repoRoot string, doc KnowledgeDoc, docIDs map[string]struct{}) {
	steps, ok := doc.Raw["steps"].([]any)
	if !ok {
		return
	}
	flowID := graphDocNodeID(doc)
	for i, item := range steps {
		step, ok := item.(map[string]any)
		if !ok {
			continue
		}
		stepID := graphNodeID("step", fmt.Sprintf("%s.%d", doc.ID, i+1))
		b.addNode(GraphNode{
			ID:    stepID,
			Type:  "step",
			Label: firstNonEmpty(stringField(step, "name"), fmt.Sprintf("step %d", i+1)),
			Path:  relOrSame(repoRoot, doc.Path),
		})
		b.addEdge(GraphEdge{Source: flowID, Target: stepID, Type: "contains_step"})
		for _, rpc := range rpcRefsFromValue(item) {
			b.addEdge(GraphEdge{Source: stepID, Target: graphNodeID("rpc", rpc), Type: "uses_rpc"})
		}
		if service := stringField(step, "service"); service != "" {
			serviceID := graphNodeID("service", service)
			b.addNode(GraphNode{ID: serviceID, Type: "service", Label: service})
			b.addEdge(GraphEdge{Source: stepID, Target: serviceID, Type: "runs_in_service"})
		}
		if externalSystem := stringField(step, "external_system"); externalSystem != "" {
			targetID := graphNodeID("integration", externalSystem)
			if _, ok := docIDs[targetID]; !ok {
				targetID = graphNodeID("external_system", externalSystem)
				b.addNode(GraphNode{ID: targetID, Type: "external_system", Label: externalSystem})
			}
			b.addEdge(GraphEdge{Source: stepID, Target: targetID, Type: "external_system"})
		}
	}
}

func addRPCEdges(builder graphBuilder, sourceID string, raw map[string]any, field string) {
	for _, rpc := range rpcRefsFromValue(raw[field]) {
		builder.addEdge(GraphEdge{Source: sourceID, Target: graphNodeID("rpc", rpc), Type: field})
	}
}

func addAvoidRPCEdges(builder graphBuilder, sourceID string, v any) {
	for _, entry := range avoidRPCEntries(v) {
		builder.addEdge(GraphEdge{
			Source: sourceID,
			Target: graphNodeID("rpc", entry.RPC),
			Type:   "avoid_rpc",
			Reason: entry.Reason,
		})
	}
}

func avoidRPCSet(raw map[string]any) map[string]struct{} {
	out := map[string]struct{}{}
	for _, v := range []any{raw["avoid_rpc"], raw["avoid_rpcs"]} {
		for _, entry := range avoidRPCEntries(v) {
			if entry.RPC != "" {
				out[entry.RPC] = struct{}{}
			}
		}
	}
	return out
}

func rpcRefsFromValue(v any) []string {
	out := map[string]struct{}{}
	collectRPCValue(v, out)
	return sortedKeys(out)
}

func validateContextGraph(graph ContextGraph) []ValidationIssue {
	var issues []ValidationIssue
	nodes := map[string]struct{}{}
	for _, node := range graph.Nodes {
		if node.ID == "" {
			issues = append(issues, ValidationIssue{"error", "", "graph node missing id"})
			continue
		}
		if node.Type == "" {
			issues = append(issues, ValidationIssue{"error", "", "graph node " + node.ID + " missing type"})
		}
		if _, ok := nodes[node.ID]; ok {
			issues = append(issues, ValidationIssue{"error", "", "duplicate graph node " + node.ID})
		}
		nodes[node.ID] = struct{}{}
	}
	for _, edge := range graph.Edges {
		if edge.Source == "" || edge.Target == "" || edge.Type == "" {
			issues = append(issues, ValidationIssue{"error", "", "graph edge missing source, target or type"})
			continue
		}
		if _, ok := nodes[edge.Source]; !ok {
			issues = append(issues, ValidationIssue{"error", "", "graph edge source not found: " + edge.Source})
		}
		if _, ok := nodes[edge.Target]; !ok {
			issues = append(issues, ValidationIssue{"error", "", "graph edge target not found: " + edge.Target})
		}
	}
	return issues
}

func graphDocNodeID(doc KnowledgeDoc) string {
	return graphNodeID(doc.Type, doc.ID)
}

func graphNodeID(typ, id string) string {
	return typ + ":" + id
}

func sortedRPCNames(index map[string]RPCDefinition) []string {
	names := make([]string, 0, len(index))
	for name := range index {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
