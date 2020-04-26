package tools

// Example:
// Not loadable tool.

// ToolNotRecognized is a struct example that does not implement
// the `Configurable`, nor the `Factory` interface natively and
// nor it has a registered `FactoryFunc`.
type ToolNotRecognized struct {
	Text string `yaml:"text"`
}

// GetText returns the text stored in Tool.
func (t *ToolNotRecognized) GetText() string {
	return t.Text
}
