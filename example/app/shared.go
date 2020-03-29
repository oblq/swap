package app

import (
	"reflect"

	"github.com/oblq/swap"
	"github.com/oblq/swap/example/app/tools"
)

// ToolBox is the struct to initialize.
var ToolBox struct {
	// By default, swap will look for a file named like the
	// struct field name (Tool.*, case-sensitive).
	Tool1 tools.ToolConfigurable
	Tool2 tools.ToolWFactory
	Tool3 tools.ToolRegistered
	Tool4 tools.ToolNotRecognized

	Nested1 struct {
		Tool1   tools.ToolConfigurable
		Nested2 struct {
			Tool2   tools.ToolConfigurable
			Nested3 struct {
				Tool3 tools.ToolConfigurable
			}
		}
	}

	// MediaProcessing does not implement the 'Configurable'
	// nor the `Factory` interface, so it will be just traversed recursively.
	// Recursion will only stop when no more embedded elements are found
	// or when a 'Factory' interface is found or
	// when a type for which a `FactoryFunc` has been registered.
	// Recursion will work in reverse order: the deepest fields
	// will be analyzed (configured/made) first.
	MediaProcessing struct {
		// Optionally pass one or more config file name in the tag,
		// file extension can be omitted.
		Pictures tools.Service `swap:"mp_dir/Pictures|mp_dir/PicturesOverride"`
		Videos   tools.Service `swap:"mp_dir/Videos"`
	}

	// Add the '-' value to skip the field.
	OmittedTool        tools.ToolConfigurable `swap:"-"`
	ManuallyConfigured tools.ToolConfigurable
}

// EnvHandler is a customised environment handler.
var EnvHandler *swap.EnvironmentHandler

func init() {
	// Initialize a custom EnvironmentHandler which will include
	// a custom environment.
	recognizableEnvironments := append(swap.DefaultEnvs.Slice(),
		swap.NewEnvironment("my_custom_env", `(my_custom_env)|(custom)`),
	)
	EnvHandler = swap.NewEnvironmentHandler(recognizableEnvironments)

	// Default environments regexp can be edited at any time.
	//swap.DefaultEnvs.Production.regexp = regexp.MustCompile(`(my_custom_env)|(custom)`)

	// Get a new instance of swap with our custom *environmentHandler
	var builder = swap.NewBuilder("./config").WithCustomEnvHandler(EnvHandler)

	// Set the current build environment manually, our `custom` one.
	builder.EnvHandler.SetCurrent("custom")

	// Show unhandled and skipped fields just for debug purpose.
	builder.DebugOptions.HideSkipped = false
	builder.DebugOptions.HideUnhandled = false

	// Register the `FactoryFunc` for the `ToolRegistered` type
	// since it does not implement the `Factory` interface nor
	// the `Configurable` one and hence it will remain unhandled otherwise.
	builder.RegisterType(reflect.TypeOf(tools.ToolRegistered{}),
		func(configFiles ...string) (i interface{}, err error) {
			instance := &tools.ToolRegistered{}
			err = swap.Parse(&instance, configFiles...)
			return instance, err
		})

	// ManuallyConfigured configured manually...
	ToolBox.ManuallyConfigured = tools.ToolConfigurable{Text: "manually set"}

	// Load the toolbox
	if err := builder.Build(&ToolBox); err != nil {
		panic(err)
	}
}
