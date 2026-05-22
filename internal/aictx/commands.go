package aictx

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}
	switch args[0] {
	case "init":
		return cmdInit(args[1:], stdout, stderr)
	case "resolve":
		return cmdResolve(args[1:], stdout, stderr)
	case "validate":
		return cmdValidate(args[1:], stdout, stderr)
	case "query":
		return cmdQuery(args[1:], stdout, stderr)
	case "pack":
		return cmdPack(args[1:], stdout, stderr)
	case "add":
		return cmdAdd(args[1:], stdout, stderr)
	case "accept":
		return cmdAccept(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `ai-context stage 1

Usage:
  ai-context init [--force]
  ai-context resolve [--config .ai-context/config.json] Service.Method
  ai-context validate [--config .ai-context/config.json]
  ai-context query [--config .ai-context/config.json] [--limit 10] "业务词或 PRD 片段"
  ai-context pack [--config .ai-context/config.json] [--limit 5] "任务描述"
  ai-context add [--config .ai-context/config.json] --type decision --id id --title title "note"
  ai-context accept [--config .ai-context/config.json] .ai-context/pending/file.json`)
}

func cmdInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	force := fs.Bool("force", false, "overwrite existing files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	root, _ := os.Getwd()
	ctxRoot := filepath.Join(root, ".ai-context")
	dirs := []string{"capabilities", "rpc-mapping", "flows", "integrations", "runbooks", "decisions", "terms", "pending", "templates"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(ctxRoot, dir), 0o755); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	if err := writeJSONFile(filepath.Join(ctxRoot, "config.json"), defaultConfig(), *force); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	templates := map[string]map[string]any{
		"capability.json": {
			"type":            "capability",
			"id":              "domain.capability",
			"title":           "能力名称",
			"aliases":         []string{"业务别名"},
			"prd_keywords":    []string{"PRD 关键词"},
			"owning_services": []string{"service-name"},
			"related_actions": []string{"domain.action"},
			"related_flows":   []string{"domain.flow"},
		},
		"rpc-mapping.json": {
			"type":                 "rpc_mapping",
			"id":                   "domain.action.rpc",
			"title":                "业务动作接口映射",
			"business_action":      "业务动作",
			"aliases":              []string{"业务别名"},
			"primary_rpc":          []string{"Service.Method"},
			"related_capabilities": []string{"domain.capability"},
			"avoid_rpc":            []map[string]string{{"rpc": "Service.OtherMethod", "reason": "不适用原因"}},
		},
		"flow.json": {
			"type":      "flow",
			"id":        "domain.flow",
			"title":     "流程名称",
			"flow_type": "technical",
			"steps":     []map[string]string{{"name": "步骤", "rpc": "Service.Method"}},
		},
		"integration.json": {
			"type":           "integration",
			"id":             "external-system",
			"title":          "外部系统名称",
			"request_entry":  map[string]string{"rpc": "Service.Method"},
			"callback_entry": map[string]string{"rpc": "Service.Callback"},
			"contracts":      []string{"对接契约"},
		},
		"runbook.json": {
			"type":                 "runbook",
			"id":                   "incident-scenario",
			"title":                "排查手册名称",
			"symptoms":             []string{"现象"},
			"checks":               []string{"排查项"},
			"related_flows":        []string{"domain.flow"},
			"related_integrations": []string{"external-system"},
		},
	}
	for name, tmpl := range templates {
		if err := writeJSONFile(filepath.Join(ctxRoot, "templates", name), tmpl, *force); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "initialized %s\n", ctxRoot)
	return 0
}

func cmdResolve(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("resolve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "config path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "resolve requires Service.Method")
		return 2
	}
	cfg, repoRoot, rpcIndex, ok := loadRuntime(*configPath, stderr)
	if !ok {
		return 1
	}
	_ = cfg
	def, exists := rpcIndex[fs.Arg(0)]
	if !exists {
		fmt.Fprintf(stderr, "RPC not found: %s\n", fs.Arg(0))
		return 1
	}
	def.ThriftFile = relOrSame(repoRoot, def.ThriftFile)
	printJSON(stdout, def)
	return 0
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "config path")
	pending := fs.Bool("pending", false, "include pending files")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, repoRoot, rpcIndex, ok := loadRuntime(*configPath, stderr)
	if !ok {
		return 1
	}
	docs, err := loadKnowledge(resolvePath(repoRoot, cfg.Knowledge.Root), *pending)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	issues := validateKnowledge(docs, rpcIndex)
	for _, issue := range issues {
		fmt.Fprintf(stdout, "%s %s: %s\n", strings.ToUpper(issue.Level), relOrSame(repoRoot, issue.Path), issue.Message)
	}
	if hasErrors(issues) {
		fmt.Fprintf(stdout, "validation failed: %d issue(s)\n", len(issues))
		return 1
	}
	fmt.Fprintf(stdout, "validation passed: %d knowledge file(s), %d RPC(s)\n", len(docs), len(rpcIndex))
	return 0
}

func cmdQuery(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "config path")
	limit := fs.Int("limit", 10, "max results")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	query := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(stderr, "query requires text")
		return 2
	}
	cfg, repoRoot, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	docs, err := loadKnowledge(resolvePath(repoRoot, cfg.Knowledge.Root), false)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	results := searchDocs(docs, query, *limit)
	if len(results) == 0 {
		fmt.Fprintf(stdout, "no match for %q\n", query)
		return 0
	}
	for i, result := range results {
		doc := result.Doc
		fmt.Fprintf(stdout, "%d. [%s] %s", i+1, doc.Type, doc.ID)
		if doc.Title != "" {
			fmt.Fprintf(stdout, " - %s", doc.Title)
		}
		fmt.Fprintf(stdout, " (score=%d)\n", result.Score)
		if len(doc.Reason) > 0 {
			fmt.Fprintf(stdout, "   match: %s\n", strings.Join(doc.Reason, ", "))
		}
		if len(doc.RPCs) > 0 {
			fmt.Fprintf(stdout, "   rpc: %s\n", strings.Join(doc.RPCs, ", "))
		}
		fmt.Fprintf(stdout, "   file: %s\n", relOrSame(repoRoot, doc.Path))
	}
	return 0
}

func cmdPack(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "config path")
	limit := fs.Int("limit", 5, "max seed results")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	task := strings.Join(fs.Args(), " ")
	if strings.TrimSpace(task) == "" {
		fmt.Fprintln(stderr, "pack requires task text")
		return 2
	}
	cfg, repoRoot, rpcIndex, ok := loadRuntime(*configPath, stderr)
	if !ok {
		return 1
	}
	docs, err := loadKnowledge(resolvePath(repoRoot, cfg.Knowledge.Root), false)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	selected := expandResults(searchDocs(docs, task, *limit), docs)
	fmt.Fprintf(stdout, "# AI Context Pack\n\n")
	fmt.Fprintf(stdout, "Task: %s\n\n", task)
	if len(selected) == 0 {
		fmt.Fprintln(stdout, "No matching knowledge found.")
		return 0
	}
	fmt.Fprintln(stdout, "## Matched Knowledge")
	for _, doc := range selected {
		fmt.Fprintf(stdout, "- [%s] %s", doc.Type, doc.ID)
		if doc.Title != "" {
			fmt.Fprintf(stdout, ": %s", doc.Title)
		}
		fmt.Fprintf(stdout, " (%s)\n", relOrSame(repoRoot, doc.Path))
	}
	avoidRPCs := collectAvoidRPCsFromDocs(selected)
	rpcs := collectRPCsFromDocs(selected, avoidRPCs)
	if len(rpcs) > 0 {
		fmt.Fprintln(stdout, "\n## RPCs")
		for _, rpc := range rpcs {
			if def, ok := rpcIndex[rpc]; ok {
				fmt.Fprintf(stdout, "- %s\n", rpc)
				fmt.Fprintf(stdout, "  - thrift: %s\n", relOrSame(repoRoot, def.ThriftFile))
				if def.Request != "" {
					fmt.Fprintf(stdout, "  - request: %s\n", def.Request)
				}
				if def.Response != "" {
					fmt.Fprintf(stdout, "  - response: %s\n", def.Response)
				}
				if def.MethodComment != "" {
					fmt.Fprintf(stdout, "  - comment: %s\n", oneLine(def.MethodComment))
				}
			} else {
				fmt.Fprintf(stdout, "- %s (UNKNOWN)\n", rpc)
			}
		}
	}
	if len(avoidRPCs) > 0 {
		fmt.Fprintln(stdout, "\n## Avoid RPCs")
		for _, entry := range avoidRPCs {
			if entry.Reason == "" {
				fmt.Fprintf(stdout, "- %s\n", entry.RPC)
			} else {
				fmt.Fprintf(stdout, "- %s: %s\n", entry.RPC, entry.Reason)
			}
		}
	}
	fmt.Fprintln(stdout, "\n## Notes")
	for _, doc := range selected {
		for _, note := range stringList(doc.Raw["notes"]) {
			fmt.Fprintf(stdout, "- %s: %s\n", doc.ID, note)
		}
		for _, risk := range stringList(doc.Raw["risk"]) {
			fmt.Fprintf(stdout, "- %s risk: %s\n", doc.ID, risk)
		}
		for _, item := range stringList(doc.Raw["contracts"]) {
			fmt.Fprintf(stdout, "- %s contract: %s\n", doc.ID, item)
		}
	}
	return 0
}

func cmdAdd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "config path")
	docType := fs.String("type", "decision", "knowledge type")
	id := fs.String("id", "", "knowledge id")
	title := fs.String("title", "", "title")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	note := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if *id == "" || *title == "" || note == "" {
		fmt.Fprintln(stderr, "add requires --id, --title and note text")
		return 2
	}
	cfg, repoRoot, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	doc := map[string]any{
		"type":       *docType,
		"id":         *id,
		"title":      *title,
		"notes":      []string{note},
		"created_at": time.Now().Format(time.RFC3339),
		"status":     "pending",
	}
	name := sanitizeFileName(*id) + ".json"
	path := filepath.Join(resolvePath(repoRoot, cfg.Knowledge.Root), "pending", name)
	if err := writeJSONFile(path, doc, false); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "created pending knowledge: %s\n", relOrSame(repoRoot, path))
	return 0
}

func cmdAccept(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("accept", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", defaultConfigPath, "config path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "accept requires pending file path")
		return 2
	}
	cfg, repoRoot, rpcIndex, ok := loadRuntime(*configPath, stderr)
	if !ok {
		return 1
	}
	pendingPath := resolvePath(repoRoot, fs.Arg(0))
	doc, err := readKnowledgeDoc(pendingPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	dir, ok := typeToDir[doc.Type]
	if !ok {
		fmt.Fprintf(stderr, "unknown type %s\n", doc.Type)
		return 1
	}
	root := resolvePath(repoRoot, cfg.Knowledge.Root)
	existing, err := loadKnowledge(root, false)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	toValidate := append(existing, doc)
	issues := validateKnowledge(toValidate, rpcIndex)
	for _, issue := range issues {
		fmt.Fprintf(stdout, "%s %s: %s\n", strings.ToUpper(issue.Level), relOrSame(repoRoot, issue.Path), issue.Message)
	}
	if hasErrors(issues) {
		fmt.Fprintln(stdout, "accept aborted: validation failed")
		return 1
	}
	dest := filepath.Join(root, dir, sanitizeFileName(doc.ID)+".json")
	if _, err := os.Stat(dest); err == nil {
		fmt.Fprintf(stderr, "destination already exists: %s\n", relOrSame(repoRoot, dest))
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	delete(doc.Raw, "status")
	doc.Raw["accepted_at"] = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(doc.Raw, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	data = append(data, '\n')
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := os.Remove(pendingPath); err != nil {
		fmt.Fprintf(stderr, "accepted but failed to remove pending file: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "accepted %s -> %s\n", relOrSame(repoRoot, pendingPath), relOrSame(repoRoot, dest))
	return 0
}

func loadRuntime(configPath string, stderr io.Writer) (Config, string, map[string]RPCDefinition, bool) {
	cfg, repoRoot, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return Config{}, "", nil, false
	}
	rpcIndex, err := loadRPCIndex(cfg, repoRoot)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return Config{}, "", nil, false
	}
	return cfg, repoRoot, rpcIndex, true
}

func printJSON(w io.Writer, v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(w, string(data))
}

func relOrSame(base, path string) string {
	if path == "" {
		return ""
	}
	rel, err := filepath.Rel(base, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return path
	}
	return rel
}

func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-", "*", "-", "?", "-", "\"", "-", "<", "-", ">", "-", "|", "-")
	return replacer.Replace(s)
}

func expandResults(results []QueryResult, docs []KnowledgeDoc) []KnowledgeDoc {
	byID := map[string]KnowledgeDoc{}
	for _, doc := range docs {
		byID[doc.ID] = doc
	}
	selected := map[string]KnowledgeDoc{}
	var ordered []KnowledgeDoc
	add := func(doc KnowledgeDoc) {
		if _, ok := selected[doc.ID]; ok {
			return
		}
		selected[doc.ID] = doc
		ordered = append(ordered, doc)
	}
	for queue := resultsToDocs(results); len(queue) > 0; {
		doc := queue[0]
		queue = queue[1:]
		if _, ok := selected[doc.ID]; ok {
			continue
		}
		add(doc)
		for _, key := range []string{"related_actions", "related_flows", "related_integrations", "related_runbooks", "related_capabilities", "related_decisions", "related_terms"} {
			for _, ref := range collectIDRefs(doc.Raw, key) {
				if related, ok := byID[ref]; ok {
					queue = append(queue, related)
				}
			}
		}
	}
	return ordered
}

func resultsToDocs(results []QueryResult) []KnowledgeDoc {
	out := make([]KnowledgeDoc, 0, len(results))
	for _, result := range results {
		out = append(out, result.Doc)
	}
	return out
}

func collectRPCsFromDocs(docs []KnowledgeDoc, avoid []avoidRPCEntry) []string {
	set := map[string]struct{}{}
	avoidSet := map[string]struct{}{}
	for _, entry := range avoid {
		avoidSet[entry.RPC] = struct{}{}
	}
	for _, doc := range docs {
		for _, rpc := range doc.RPCs {
			if _, skip := avoidSet[rpc]; skip {
				continue
			}
			set[rpc] = struct{}{}
		}
	}
	out := sortedKeys(set)
	sort.Strings(out)
	return out
}

type avoidRPCEntry struct {
	RPC    string
	Reason string
}

func collectAvoidRPCsFromDocs(docs []KnowledgeDoc) []avoidRPCEntry {
	byRPC := map[string]avoidRPCEntry{}
	for _, doc := range docs {
		for _, entry := range avoidRPCEntries(doc.Raw["avoid_rpc"]) {
			if entry.RPC == "" {
				continue
			}
			if existing, ok := byRPC[entry.RPC]; ok && existing.Reason != "" {
				continue
			}
			byRPC[entry.RPC] = entry
		}
	}
	keys := make([]string, 0, len(byRPC))
	for rpc := range byRPC {
		keys = append(keys, rpc)
	}
	sort.Strings(keys)
	out := make([]avoidRPCEntry, 0, len(keys))
	for _, rpc := range keys {
		out = append(out, byRPC[rpc])
	}
	return out
}

func avoidRPCEntries(v any) []avoidRPCEntry {
	switch x := v.(type) {
	case string:
		if looksLikeRPC(x) {
			return []avoidRPCEntry{{RPC: x}}
		}
	case []any:
		var out []avoidRPCEntry
		for _, item := range x {
			out = append(out, avoidRPCEntries(item)...)
		}
		return out
	case map[string]any:
		rpc, _ := x["rpc"].(string)
		if !looksLikeRPC(rpc) {
			return nil
		}
		reason, _ := x["reason"].(string)
		return []avoidRPCEntry{{RPC: rpc, Reason: reason}}
	}
	return nil
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
