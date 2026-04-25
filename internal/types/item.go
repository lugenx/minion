package types

// Item is the universal data payload that travels down the pipeline.
type Item struct {
	URL     string
	Text    string
	Summary string
	Title   string
}
