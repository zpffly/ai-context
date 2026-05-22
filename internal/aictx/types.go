package aictx

type Config struct {
	Version   int             `json:"version"`
	IDL       IDLConfig       `json:"idl"`
	Knowledge KnowledgeConfig `json:"knowledge"`
}

type IDLConfig struct {
	Root    string   `json:"root"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude,omitempty"`
}

type KnowledgeConfig struct {
	Root string `json:"root"`
}

type RPCDefinition struct {
	Service        string `json:"service"`
	Method         string `json:"method"`
	Request        string `json:"request,omitempty"`
	Response       string `json:"response,omitempty"`
	ThriftFile     string `json:"thrift_file"`
	ServiceComment string `json:"service_comment,omitempty"`
	MethodComment  string `json:"method_comment,omitempty"`
}

func (r RPCDefinition) FullName() string {
	return r.Service + "." + r.Method
}

type KnowledgeDoc struct {
	Type   string         `json:"type"`
	ID     string         `json:"id"`
	Title  string         `json:"title,omitempty"`
	Path   string         `json:"-"`
	Raw    map[string]any `json:"-"`
	RPCs   []string       `json:"-"`
	Score  int            `json:"-"`
	Reason []string       `json:"-"`
}

type ValidationIssue struct {
	Level   string
	Path    string
	Message string
}

type QueryResult struct {
	Doc          KnowledgeDoc
	Score        int
	MatchReasons []MatchReason
}

type MatchReason struct {
	Field  string   `json:"field"`
	Value  string   `json:"value,omitempty"`
	Tokens []string `json:"tokens,omitempty"`
	Score  int      `json:"score"`
}

type QueryOutput struct {
	SchemaVersion string       `json:"schema_version"`
	Query         QueryInfo    `json:"query"`
	Summary       QuerySummary `json:"summary"`
	Matches       []QueryMatch `json:"matches"`
	Warnings      []string     `json:"warnings"`
}

type QueryInfo struct {
	Text           string   `json:"text"`
	Tokens         []string `json:"tokens"`
	Limit          int      `json:"limit"`
	IncludePending bool     `json:"include_pending"`
	IncludeRaw     bool     `json:"include_raw"`
}

type QuerySummary struct {
	TotalMatches      int            `json:"total_matches"`
	HasConfidentMatch bool           `json:"has_confident_match"`
	TopMatch          *QueryTopMatch `json:"top_match,omitempty"`
}

type QueryTopMatch struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Score int    `json:"score"`
}

type QueryMatch struct {
	Rank         int            `json:"rank"`
	Score        int            `json:"score"`
	Confidence   string         `json:"confidence"`
	Doc          QueryDoc       `json:"doc"`
	MatchReasons []MatchReason  `json:"match_reasons"`
	Relations    QueryRelations `json:"relations"`
	RPCs         QueryRPCs      `json:"rpcs"`
	Snippets     []QuerySnippet `json:"snippets"`
	Raw          map[string]any `json:"raw,omitempty"`
}

type QueryDoc struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	Path  string `json:"path"`
}

type QueryRelations struct {
	RelatedCapabilities []string `json:"related_capabilities"`
	RelatedFlows        []string `json:"related_flows"`
	RelatedIntegrations []string `json:"related_integrations"`
	RelatedRunbooks     []string `json:"related_runbooks"`
	RelatedActions      []string `json:"related_actions"`
	RelatedDecisions    []string `json:"related_decisions"`
	RelatedTerms        []string `json:"related_terms"`
}

type QueryRPCs struct {
	Positive []QueryRPCRef `json:"positive"`
	Avoid    []QueryRPCRef `json:"avoid"`
	Unknown  []QueryRPCRef `json:"unknown"`
}

type QueryRPCRef struct {
	Name        string `json:"name"`
	SourceDoc   string `json:"source_doc"`
	SourceField string `json:"source_field,omitempty"`
	Reason      string `json:"reason,omitempty"`
	ThriftFile  string `json:"thrift_file,omitempty"`
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
}

type QuerySnippet struct {
	Field         string   `json:"field"`
	Text          string   `json:"text"`
	MatchedTokens []string `json:"matched_tokens"`
}

type ContextGraph struct {
	Version int         `json:"version"`
	Kind    string      `json:"kind"`
	Nodes   []GraphNode `json:"nodes"`
	Edges   []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`
	Path  string `json:"path,omitempty"`
}

type GraphEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Reason string `json:"reason,omitempty"`
}
