package tools

import "github.com/oblq/swap"

// Example:
// How to load a package which need more
// control during initialization.

// Tool is a struct example that implement
// the `Configurable` interface natively.
type ToolWFactory struct {
	Text string `yaml:"text"`
}

// New is the 'Factory' interface implementation.
func (t *ToolWFactory) New(configFiles ...string) (obj interface{}, err error) {
	instance := &ToolWFactory{}
	err = swap.Parse(&instance, configFiles...)
	return instance, err
}

// GetText returns the text stored in Tool.
func (t *ToolWFactory) GetText() string {
	return t.Text
}
