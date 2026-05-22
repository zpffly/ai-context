package aictx

import (
	"fmt"
	"sort"
	"strings"
)

const querySchemaVersion = "ai-context.query.v1"

type rpcRefEntry struct {
	RPC         string
	SourceField string
	Reason      string
}

type stringFieldEntry struct {
	Field string
	Text  string
}

func buildQueryOutput(repoRoot, query string, limit int, includePending, includeRaw bool, totalMatches int, results []QueryResult, rpcIndex map[string]RPCDefinition) QueryOutput {
	out := QueryOutput{
		SchemaVersion: querySchemaVersion,
		Query: QueryInfo{
			Text:           query,
			Tokens:         queryTokens(query),
			Limit:          limit,
			IncludePending: includePending,
			IncludeRaw:     includeRaw,
		},
		Summary: QuerySummary{
			TotalMatches: totalMatches,
		},
		Matches:  []QueryMatch{},
		Warnings: []string{},
	}
	for i, result := range results {
		match := buildQueryMatch(repoRoot, query, i+1, result, rpcIndex, includeRaw)
		out.Matches = append(out.Matches, match)
	}
	if len(out.Matches) > 0 {
		top := out.Matches[0]
		out.Summary.TopMatch = &QueryTopMatch{
			Type:  top.Doc.Type,
			ID:    top.Doc.ID,
			Score: top.Score,
		}
		out.Summary.HasConfidentMatch = top.Confidence == "high"
	}
	return out
}

func limitQueryResults(results []QueryResult, limit int) []QueryResult {
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func buildQueryMatch(repoRoot, query string, rank int, result QueryResult, rpcIndex map[string]RPCDefinition, includeRaw bool) QueryMatch {
	doc := result.Doc
	match := QueryMatch{
		Rank:       rank,
		Score:      result.Score,
		Confidence: queryConfidence(result.Score, result.MatchReasons),
		Doc: QueryDoc{
			Type:  doc.Type,
			ID:    doc.ID,
			Title: doc.Title,
			Path:  relOrSame(repoRoot, doc.Path),
		},
		MatchReasons: result.MatchReasons,
		Relations:    queryRelations(doc.Raw),
		RPCs:         queryRPCsForDoc(doc, repoRoot, rpcIndex),
		Snippets:     querySnippets(doc, query, 5),
	}
	if includeRaw {
		match.Raw = doc.Raw
	}
	return match
}

func queryConfidence(score int, reasons []MatchReason) string {
	if score >= 60 {
		return "high"
	}
	for _, reason := range reasons {
		switch baseField(reason.Field) {
		case "id", "title", "business_action", "aliases", "prd_keywords":
			if reason.Score >= 25 {
				return "high"
			}
		}
	}
	if score >= 25 {
		return "medium"
	}
	return "low"
}

func baseField(field string) string {
	field = strings.Split(field, "[")[0]
	field = strings.Split(field, ".")[0]
	return field
}

func queryRelations(raw map[string]any) QueryRelations {
	return QueryRelations{
		RelatedCapabilities: nonNilStrings(collectIDRefs(raw, "related_capabilities")),
		RelatedFlows:        nonNilStrings(collectIDRefs(raw, "related_flows")),
		RelatedIntegrations: nonNilStrings(collectIDRefs(raw, "related_integrations")),
		RelatedRunbooks:     nonNilStrings(collectIDRefs(raw, "related_runbooks")),
		RelatedActions:      nonNilStrings(collectIDRefs(raw, "related_actions")),
		RelatedDecisions:    nonNilStrings(collectIDRefs(raw, "related_decisions")),
		RelatedTerms:        nonNilStrings(collectIDRefs(raw, "related_terms")),
	}
}

func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func queryRPCsForDoc(doc KnowledgeDoc, repoRoot string, rpcIndex map[string]RPCDefinition) QueryRPCs {
	out := QueryRPCs{
		Positive: []QueryRPCRef{},
		Avoid:    []QueryRPCRef{},
		Unknown:  []QueryRPCRef{},
	}
	avoidRefs := collectAvoidRPCRefsWithSource(doc.Raw)
	avoidByName := map[string]struct{}{}
	for _, ref := range avoidRefs {
		avoidByName[ref.RPC] = struct{}{}
		qref := queryRPCRef(doc.ID, repoRoot, ref, rpcIndex)
		if _, ok := rpcIndex[ref.RPC]; ok {
			out.Avoid = append(out.Avoid, qref)
		} else {
			out.Unknown = append(out.Unknown, qref)
		}
	}

	for _, ref := range collectRPCRefsWithSource(doc.Raw) {
		if _, skip := avoidByName[ref.RPC]; skip {
			continue
		}
		qref := queryRPCRef(doc.ID, repoRoot, ref, rpcIndex)
		if _, ok := rpcIndex[ref.RPC]; ok {
			out.Positive = append(out.Positive, qref)
		} else {
			out.Unknown = append(out.Unknown, qref)
		}
	}
	out.Positive = sortQueryRPCRefs(dedupeQueryRPCRefs(out.Positive))
	out.Avoid = sortQueryRPCRefs(dedupeQueryRPCRefs(out.Avoid))
	out.Unknown = sortQueryRPCRefs(dedupeQueryRPCRefs(out.Unknown))
	return out
}

func queryRPCNames(refs []QueryRPCRef) []string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	sort.Strings(names)
	return names
}

func queryRPCNamesWithReason(refs []QueryRPCRef) []string {
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		name := ref.Name
		if ref.Reason != "" {
			name += " (" + ref.Reason + ")"
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func queryRPCRef(sourceDoc, repoRoot string, ref rpcRefEntry, rpcIndex map[string]RPCDefinition) QueryRPCRef {
	out := QueryRPCRef{
		Name:        ref.RPC,
		SourceDoc:   sourceDoc,
		SourceField: ref.SourceField,
		Reason:      ref.Reason,
	}
	if def, ok := rpcIndex[ref.RPC]; ok {
		out.ThriftFile = relOrSame(repoRoot, def.ThriftFile)
		out.Request = def.Request
		out.Response = def.Response
	}
	return out
}

func collectRPCRefsWithSource(raw map[string]any) []rpcRefEntry {
	var refs []rpcRefEntry
	collectRPCRefsWithSourceInto(raw, "", "", &refs)
	return dedupeRPCRefEntries(refs)
}

func collectRPCRefsWithSourceInto(v any, path, key string, refs *[]rpcRefEntry) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			childPath := joinFieldPath(path, k)
			lower := strings.ToLower(k)
			if isRPCField(lower) {
				*refs = append(*refs, rpcRefsFromValueWithSource(x[k], childPath)...)
				continue
			}
			collectRPCRefsWithSourceInto(x[k], childPath, lower, refs)
		}
	case []any:
		for i, item := range x {
			collectRPCRefsWithSourceInto(item, indexFieldPath(path, i), key, refs)
		}
	}
}

func rpcRefsFromValueWithSource(v any, path string) []rpcRefEntry {
	switch x := v.(type) {
	case string:
		if looksLikeRPC(x) {
			return []rpcRefEntry{{RPC: x, SourceField: path}}
		}
	case []any:
		var refs []rpcRefEntry
		for i, item := range x {
			refs = append(refs, rpcRefsFromValueWithSource(item, indexFieldPath(path, i))...)
		}
		return refs
	case map[string]any:
		if s, ok := x["rpc"].(string); ok && looksLikeRPC(s) {
			return []rpcRefEntry{{RPC: s, SourceField: joinFieldPath(path, "rpc")}}
		}
	}
	return nil
}

func collectAvoidRPCRefsWithSource(raw map[string]any) []rpcRefEntry {
	var refs []rpcRefEntry
	for _, field := range []string{"avoid_rpc", "avoid_rpcs"} {
		refs = append(refs, avoidRPCRefsFromValueWithSource(raw[field], field)...)
	}
	return dedupeRPCRefEntries(refs)
}

func avoidRPCRefsFromValueWithSource(v any, path string) []rpcRefEntry {
	switch x := v.(type) {
	case string:
		if looksLikeRPC(x) {
			return []rpcRefEntry{{RPC: x, SourceField: path}}
		}
	case []any:
		var refs []rpcRefEntry
		for i, item := range x {
			refs = append(refs, avoidRPCRefsFromValueWithSource(item, indexFieldPath(path, i))...)
		}
		return refs
	case map[string]any:
		rpc, _ := x["rpc"].(string)
		if !looksLikeRPC(rpc) {
			return nil
		}
		reason, _ := x["reason"].(string)
		return []rpcRefEntry{{RPC: rpc, SourceField: joinFieldPath(path, "rpc"), Reason: reason}}
	}
	return nil
}

func isRPCField(lower string) bool {
	return lower == "rpc" || lower == "rpcs" || strings.HasSuffix(lower, "_rpc") || strings.HasSuffix(lower, "_rpcs")
}

func dedupeRPCRefEntries(refs []rpcRefEntry) []rpcRefEntry {
	seen := map[string]struct{}{}
	var out []rpcRefEntry
	for _, ref := range refs {
		if ref.RPC == "" {
			continue
		}
		key := ref.RPC + "\x00" + ref.SourceField + "\x00" + ref.Reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RPC != out[j].RPC {
			return out[i].RPC < out[j].RPC
		}
		if out[i].SourceField != out[j].SourceField {
			return out[i].SourceField < out[j].SourceField
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

func dedupeQueryRPCRefs(refs []QueryRPCRef) []QueryRPCRef {
	seen := map[string]struct{}{}
	out := make([]QueryRPCRef, 0, len(refs))
	for _, ref := range refs {
		key := ref.Name + "\x00" + ref.SourceDoc + "\x00" + ref.SourceField + "\x00" + ref.Reason
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ref)
	}
	return out
}

func sortQueryRPCRefs(refs []QueryRPCRef) []QueryRPCRef {
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Name != refs[j].Name {
			return refs[i].Name < refs[j].Name
		}
		if refs[i].SourceDoc != refs[j].SourceDoc {
			return refs[i].SourceDoc < refs[j].SourceDoc
		}
		return refs[i].SourceField < refs[j].SourceField
	})
	return refs
}

func querySnippets(doc KnowledgeDoc, query string, limit int) []QuerySnippet {
	tokens := queryTokens(query)
	snippets := []QuerySnippet{}
	for _, field := range flattenStringFieldsWithPath(doc.Raw, "") {
		matched := matchedTokens(field.Text, tokens)
		if len(matched) == 0 {
			continue
		}
		snippets = append(snippets, QuerySnippet{
			Field:         field.Field,
			Text:          oneLine(field.Text),
			MatchedTokens: matched,
		})
		if limit > 0 && len(snippets) >= limit {
			return snippets
		}
	}
	return snippets
}

func flattenStringFieldsWithPath(v any, path string) []stringFieldEntry {
	switch x := v.(type) {
	case string:
		field := path
		if field == "" {
			field = "value"
		}
		return []stringFieldEntry{{Field: field, Text: x}}
	case []any:
		var fields []stringFieldEntry
		for i, item := range x {
			fields = append(fields, flattenStringFieldsWithPath(item, indexFieldPath(path, i))...)
		}
		return fields
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var fields []stringFieldEntry
		for _, k := range keys {
			fields = append(fields, flattenStringFieldsWithPath(x[k], joinFieldPath(path, k))...)
		}
		return fields
	default:
		return nil
	}
}

func matchedTokens(text string, tokens []string) []string {
	lower := strings.ToLower(text)
	var matched []string
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(token)) {
			matched = append(matched, token)
		}
		if len(matched) >= 8 {
			break
		}
	}
	return compactStrings(matched)
}

func joinFieldPath(base, field string) string {
	if base == "" {
		return field
	}
	return base + "." + field
}

func indexFieldPath(base string, idx int) string {
	if base == "" {
		return fmt.Sprintf("[%d]", idx)
	}
	return fmt.Sprintf("%s[%d]", base, idx)
}
