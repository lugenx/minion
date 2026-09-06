package types

// FileRecord is the schema for structured YAML pipeline I/O.
type FileRecord struct {
	Title     string `json:"title,omitempty" yaml:"title,omitempty"`
	URL       string `json:"url,omitempty" yaml:"url,omitempty"`
	Summary   string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Text      string `json:"text,omitempty" yaml:"text,omitempty"`
	Timestamp string `json:"timestamp,omitempty" yaml:"timestamp,omitempty"`
}

// Item is the universal data payload that travels down the pipeline.
type Item struct {
	ID         string
	URL        string
	ParentURL  string
	Text       string
	Summary    string
	Title      string
	TempHash   string
	Command    string
	FilePath   string
	Timestamp  string
	Protected  bool
	Render     bool
	SourceType string
}
