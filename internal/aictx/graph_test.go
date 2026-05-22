package aictx

import (
	"path/filepath"
	"testing"
)

func TestBuildContextGraph(t *testing.T) {
	configPath := filepath.Join("..", "..", "examples", "demo", ".ai-context", "config.json")
	cfg, repoRoot, err := loadConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	rpcIndex, err := loadRPCIndex(cfg, repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := loadKnowledge(resolvePath(repoRoot, cfg.Knowledge.Root), false)
	if err != nil {
		t.Fatal(err)
	}
	if issues := validateKnowledge(docs, rpcIndex); hasErrors(issues) {
		t.Fatalf("knowledge validation failed: %#v", issues)
	}

	graph := buildContextGraph(repoRoot, docs, rpcIndex)
	if issues := validateContextGraph(graph); hasErrors(issues) {
		t.Fatalf("graph validation failed: %#v", issues)
	}
	assertGraphNode(t, graph, "capability:merchant.onboarding")
	assertGraphNode(t, graph, "rpc:InstitutionService.VerifyInstitution")
	assertGraphNode(t, graph, "thrift_service:InstitutionService")
	assertGraphNode(t, graph, "step:institution.verify.flow.1")
	assertGraphEdge(t, graph, "capability:merchant.onboarding", "rpc_mapping:institution.verify.start", "related_actions")
	assertGraphEdge(t, graph, "rpc_mapping:institution.verify.start", "rpc:InstitutionService.VerifyInstitution", "primary_rpc")
	assertGraphEdge(t, graph, "flow:institution.verify.flow", "step:institution.verify.flow.1", "contains_step")
	assertGraphEdge(t, graph, "step:institution.verify.flow.1", "rpc:InstitutionService.VerifyInstitution", "uses_rpc")
	assertGraphEdge(t, graph, "integration:institution-center", "rpc:MerchantService.SubmitInstitutionVerifyResult", "callback_entry")
	assertGraphEdge(t, graph, "rpc_mapping:institution.verify.callback", "rpc:MerchantService.UpdateMerchant", "avoid_rpc")
	assertMissingGraphEdge(t, graph, "rpc_mapping:institution.verify.callback", "rpc:MerchantService.UpdateMerchant", "references_rpc")
}

func assertGraphNode(t *testing.T, graph ContextGraph, id string) {
	t.Helper()
	for _, node := range graph.Nodes {
		if node.ID == id {
			return
		}
	}
	t.Fatalf("missing graph node %s", id)
}

func assertGraphEdge(t *testing.T, graph ContextGraph, source, target, typ string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.Source == source && edge.Target == target && edge.Type == typ {
			return
		}
	}
	t.Fatalf("missing graph edge %s --%s--> %s", source, typ, target)
}

func assertMissingGraphEdge(t *testing.T, graph ContextGraph, source, target, typ string) {
	t.Helper()
	for _, edge := range graph.Edges {
		if edge.Source == source && edge.Target == target && edge.Type == typ {
			t.Fatalf("unexpected graph edge %s --%s--> %s", source, typ, target)
		}
	}
}
