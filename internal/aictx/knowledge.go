package aictx

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

var typeToDir = map[string]string{
	"capability":  "capabilities",
	"rpc_mapping": "rpc-mapping",
	"flow":        "flows",
	"integration": "integrations",
	"runbook":     "runbooks",
	"decision":    "decisions",
	"term":        "terms",
}

var dirsToLoad = []string{
	"capabilities",
	"rpc-mapping",
	"flows",
	"integrations",
	"runbooks",
	"decisions",
	"terms",
}

func loadKnowledge(root string, includePending bool) ([]KnowledgeDoc, error) {
	dirs := append([]string{}, dirsToLoad...)
	if includePending {
		dirs = append(dirs, "pending")
	}
	var docs []KnowledgeDoc
	for _, dir := range dirs {
		base := filepath.Join(root, dir)
		if _, err := os.Stat(base); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".json" {
				return nil
			}
			doc, err := readKnowledgeDoc(path)
			if err != nil {
				return err
			}
			docs = append(docs, doc)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, nil
}

func readKnowledgeDoc(path string) (KnowledgeDoc, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return KnowledgeDoc{}, err
	}
	var raw map[string]any
	decErr := json.Unmarshal(data, &raw)
	if decErr != nil {
		return KnowledgeDoc{}, fmt.Errorf("parse %s: %w", path, decErr)
	}
	doc := KnowledgeDoc{
		Type:  stringField(raw, "type"),
		ID:    stringField(raw, "id"),
		Title: stringField(raw, "title"),
		Path:  path,
		Raw:   raw,
	}
	doc.RPCs = sortedKeys(collectRPCRefs(raw))
	return doc, nil
}

func stringField(raw map[string]any, key string) string {
	if v, ok := raw[key].(string); ok {
		return v
	}
	return ""
}

func collectRPCRefs(v any) map[string]struct{} {
	out := map[string]struct{}{}
	collectRPCRefsInto(v, "", out)
	return out
}

func collectRPCRefsInto(v any, key string, out map[string]struct{}) {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			lower := strings.ToLower(k)
			if lower == "rpc" || lower == "rpcs" || strings.HasSuffix(lower, "_rpc") || strings.HasSuffix(lower, "_rpcs") {
				collectRPCValue(val, out)
				continue
			}
			collectRPCRefsInto(val, lower, out)
		}
	case []any:
		for _, item := range x {
			collectRPCRefsInto(item, key, out)
		}
	}
}

func collectRPCValue(v any, out map[string]struct{}) {
	switch x := v.(type) {
	case string:
		if looksLikeRPC(x) {
			out[x] = struct{}{}
		}
	case []any:
		for _, item := range x {
			collectRPCValue(item, out)
		}
	case map[string]any:
		if s, ok := x["rpc"].(string); ok && looksLikeRPC(s) {
			out[s] = struct{}{}
		}
	}
}

func looksLikeRPC(s string) bool {
	if strings.Count(s, ".") != 1 {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	return isIdent(parts[0]) && isIdent(parts[1])
}

func isIdent(s string) bool {
	for i, r := range s {
		if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

func collectIDRefs(raw map[string]any, key string) []string {
	v, ok := raw[key]
	if !ok {
		return nil
	}
	var refs []string
	switch x := v.(type) {
	case string:
		refs = append(refs, x)
	case []any:
		for _, item := range x {
			if s, ok := item.(string); ok {
				refs = append(refs, s)
			}
		}
	}
	sort.Strings(refs)
	return refs
}

func searchDocs(docs []KnowledgeDoc, query string, limit int) []QueryResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	var results []QueryResult
	for _, doc := range docs {
		score, reasons := scoreDoc(doc, query)
		if score > 0 {
			doc.Score = score
			doc.Reason = reasonLabels(reasons)
			results = append(results, QueryResult{Doc: doc, Score: score, MatchReasons: reasons})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Doc.ID < results[j].Doc.ID
		}
		return results[i].Score > results[j].Score
	})
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}

func scoreDoc(doc KnowledgeDoc, query string) (int, []MatchReason) {
	lowerQuery := strings.ToLower(query)
	score := 0
	var reasons []MatchReason
	add := func(reason MatchReason) {
		score += reason.Score
		reasons = append(reasons, reason)
	}
	if strings.EqualFold(doc.ID, query) {
		add(MatchReason{Field: "id", Value: doc.ID, Score: 100})
	}
	if containsFold(doc.Title, query) {
		add(MatchReason{Field: "title", Value: doc.Title, Score: 30})
	}
	if containsFold(stringField(doc.Raw, "business_action"), query) {
		add(MatchReason{Field: "business_action", Value: stringField(doc.Raw, "business_action"), Score: 30})
	}
	for _, key := range []string{"aliases", "prd_keywords"} {
		for i, val := range stringList(doc.Raw[key]) {
			if containsFold(val, query) || containsFold(query, val) {
				add(MatchReason{Field: fmt.Sprintf("%s[%d]", key, i), Value: val, Score: 25})
			}
		}
	}
	for _, ref := range collectRPCRefsWithSource(doc.Raw) {
		if strings.EqualFold(ref.RPC, query) || strings.Contains(strings.ToLower(ref.RPC), lowerQuery) {
			field := ref.SourceField
			if field == "" {
				field = "rpc"
			}
			add(MatchReason{Field: field, Value: ref.RPC, Score: 20})
		}
	}
	text := strings.ToLower(flattenStrings(doc.Raw))
	textScore := 0
	var matched []string
	for _, token := range queryTokens(query) {
		if token != "" && strings.Contains(text, strings.ToLower(token)) {
			textScore += 3
			matched = append(matched, token)
		}
	}
	if textScore > 0 {
		if textScore > 30 {
			textScore = 30
		}
		add(MatchReason{Field: "text", Tokens: compactStrings(matched), Score: textScore})
	}
	return score, compactMatchReasons(reasons)
}

func compactMatchReasons(in []MatchReason) []MatchReason {
	seen := map[string]struct{}{}
	var out []MatchReason
	for _, reason := range in {
		key := reason.Field + "\x00" + reason.Value + "\x00" + strings.Join(reason.Tokens, "\x00")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, reason)
	}
	return out
}

func reasonLabels(reasons []MatchReason) []string {
	var labels []string
	for _, reason := range reasons {
		switch {
		case reason.Value != "":
			labels = append(labels, reason.Field+":"+reason.Value)
		case len(reason.Tokens) > 0:
			labels = append(labels, reason.Field+":"+strings.Join(reason.Tokens, "/"))
		default:
			labels = append(labels, reason.Field)
		}
	}
	return compactStrings(labels)
}

func containsFold(haystack, needle string) bool {
	if haystack == "" || needle == "" {
		return false
	}
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func stringList(v any) []string {
	switch x := v.(type) {
	case []any:
		var out []string
		for _, item := range x {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return x
	case string:
		return []string{x}
	default:
		return nil
	}
}

func flattenStrings(v any) string {
	var parts []string
	var walk func(any)
	walk = func(x any) {
		switch t := x.(type) {
		case string:
			parts = append(parts, t)
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			for _, item := range t {
				walk(item)
			}
		}
	}
	walk(v)
	return strings.Join(parts, "\n")
}

func queryTokens(q string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(token string) {
		token = strings.TrimSpace(token)
		if token == "" {
			return
		}
		if _, ok := seen[token]; ok {
			return
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == ',' || r == '，' || r == ';' || r == '；'
	})
	for _, field := range fields {
		add(field)
		for _, token := range cjkNgrams(field) {
			add(token)
		}
	}
	if len(out) == 0 && strings.TrimSpace(q) != "" {
		add(q)
	}
	return out
}

func cjkNgrams(s string) []string {
	var out []string
	var span []rune
	flush := func() {
		if len(span) < 2 {
			span = nil
			return
		}
		if len(span) > 128 {
			span = span[:128]
		}
		for n := 2; n <= 3; n++ {
			if len(span) < n {
				continue
			}
			for i := 0; i+n <= len(span); i++ {
				out = append(out, string(span[i:i+n]))
			}
		}
		span = nil
	}
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			span = append(span, r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func compactStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range in {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func sortedKeys(m map[string]struct{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
