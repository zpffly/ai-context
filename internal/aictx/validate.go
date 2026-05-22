package aictx

import (
	"fmt"
	"path/filepath"
)

func validateKnowledge(docs []KnowledgeDoc, rpcIndex map[string]RPCDefinition) []ValidationIssue {
	var issues []ValidationIssue
	ids := map[string]KnowledgeDoc{}
	byType := map[string]map[string]KnowledgeDoc{}

	for _, doc := range docs {
		if doc.Type == "" {
			issues = append(issues, ValidationIssue{"error", doc.Path, "missing type"})
		}
		if doc.ID == "" {
			issues = append(issues, ValidationIssue{"error", doc.Path, "missing id"})
		}
		if doc.Type != "" {
			if _, ok := typeToDir[doc.Type]; !ok {
				issues = append(issues, ValidationIssue{"error", doc.Path, "unknown type " + doc.Type})
			}
		}
		if doc.ID != "" {
			if prev, exists := ids[doc.ID]; exists {
				issues = append(issues, ValidationIssue{"error", doc.Path, fmt.Sprintf("duplicate id %s, first seen at %s", doc.ID, prev.Path)})
			} else {
				ids[doc.ID] = doc
			}
		}
		if doc.Type != "" && doc.ID != "" {
			if byType[doc.Type] == nil {
				byType[doc.Type] = map[string]KnowledgeDoc{}
			}
			byType[doc.Type][doc.ID] = doc
		}
		for _, rpc := range doc.RPCs {
			if _, ok := rpcIndex[rpc]; !ok {
				issues = append(issues, ValidationIssue{"error", doc.Path, "unknown RPC reference " + rpc})
			}
		}
		if expectedDir, ok := typeToDir[doc.Type]; ok && doc.Path != "" {
			if filepath.Base(filepath.Dir(doc.Path)) != expectedDir && filepath.Base(filepath.Dir(doc.Path)) != "pending" {
				issues = append(issues, ValidationIssue{"warning", doc.Path, fmt.Sprintf("type %s usually belongs in %s", doc.Type, expectedDir)})
			}
		}
	}

	refChecks := []struct {
		field string
		typ   string
	}{
		{"related_flows", "flow"},
		{"related_actions", "rpc_mapping"},
		{"related_integrations", "integration"},
		{"related_runbooks", "runbook"},
		{"related_capabilities", "capability"},
		{"related_decisions", "decision"},
		{"related_terms", "term"},
	}
	for _, doc := range docs {
		for _, check := range refChecks {
			for _, ref := range collectIDRefs(doc.Raw, check.field) {
				if _, ok := byType[check.typ][ref]; !ok {
					issues = append(issues, ValidationIssue{"error", doc.Path, fmt.Sprintf("unknown %s reference %s in %s", check.typ, ref, check.field)})
				}
			}
		}
	}
	return issues
}

func hasErrors(issues []ValidationIssue) bool {
	for _, issue := range issues {
		if issue.Level == "error" {
			return true
		}
	}
	return false
}
