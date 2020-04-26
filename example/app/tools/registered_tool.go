package tools

// Example:
// How to load a third-party package.

// ToolRegistered is a struct example that does not implement
// the `Configurable` nor the `Factory` interface natively.
type ToolRegistered struct {
	Text string `yaml:"text"`
}

// GetText returns the text stored in Tool.
func (t *ToolRegistered) GetText() string {
	return t.Text
}

func init() {

}
