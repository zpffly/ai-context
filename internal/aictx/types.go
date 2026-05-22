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
	Doc   KnowledgeDoc
	Score int
}
