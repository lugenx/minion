package types

// Item is the universal data payload that travels down the pipeline.
type Item struct {
	ID        string
	URL       string
	ParentURL string
	Text      string
	Summary   string
	Title     string
	TempHash  string
}
