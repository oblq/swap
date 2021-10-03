package tools

import (
	"github.com/oblq/swap"
)

// Example:
// How to load a package easily and automatically.

// ToolConfigurable is a struct example that implement
// the `Configurable` interface natively.
type ToolConfigurable struct {
	Text string `yaml:"text"`
}

// Configure is the 'Configurable' interface implementation.
func (t *ToolConfigurable) Configure(configFiles ...string) (err error) {
	return swap.Parse(t, configFiles...)
}

// GetText returns the text stored in Tool.
func (t *ToolConfigurable) GetText() string {
	return t.Text
}
