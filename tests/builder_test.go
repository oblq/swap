package tests

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/oblq/swap"
	"github.com/oblq/swap/internal/logger"
	"github.com/stretchr/testify/require"
)

type ToolConfig struct {
	TestString string
}

// ---------------------------------------------------------------------------------------------------------------------

// ToolConfigurable is a struct implementing 'Configurable' interface.
type ToolConfigurable struct {
	Config ToolConfig
}

// Configure is the 'Configurable' interface implementation.
func (c *ToolConfigurable) Configure(configFiles ...string) error {
	return swap.Parse(&c.Config, configFiles...)
}

// ---------------------------------------------------------------------------------------------------------------------

// ToolMakeable is a struct implementing 'Makeable' interface.
type ToolMakeable struct {
	Config ToolConfig
}

// Configure is the 'Makeable' interface implementation.
func (c ToolMakeable) New(configFiles ...string) (obj interface{}, err error) {
	instance := ToolMakeable{}
	err = swap.Parse(&instance.Config, configFiles...)
	return instance, err
}

// ---------------------------------------------------------------------------------------------------------------------

// ToolMakeablePtr is a struct implementing 'Makeable' interface.
type ToolMakeablePtr struct {
	Config ToolConfig
}

// Configure is the 'Makeable' interface implementation.
func (c ToolMakeablePtr) New(configFiles ...string) (obj interface{}, err error) {
	instance := &ToolMakeablePtr{}
	err = swap.Parse(&instance.Config, configFiles...)
	return instance, err
}

// ---------------------------------------------------------------------------------------------------------------------

// ToolMakeableWrongReturnType is a struct implementing 'Makeable' interface.
type ToolMakeableWrongReturnType struct {
	Config ToolConfig
}

// Configure is the 'Makeable' interface implementation.
func (c ToolMakeableWrongReturnType) New(configFiles ...string) (obj interface{}, err error) {
	instance := &ToolMakeable{}
	err = swap.Parse(&instance.Config, configFiles...)
	return instance, err
}

// ---------------------------------------------------------------------------------------------------------------------

// ToolError is a struct implementing 'Configurable' interface
// which will return an error.
type ToolError struct {
	TestString string
}

// SpareConfig is the 'Configurable' interface implementation.
func (c *ToolError) Configure(...string) error {
	return errors.New("fake error for test")
}

// ---------------------------------------------------------------------------------------------------------------------

// Tool does not implement any builder interface.
type Tool struct {
	TestString string
}

// Tool2 does not implement any builder interface.
type Tool2 struct {
	TestString string
}

// ---------------------------------------------------------------------------------------------------------------------

type SubBoxConfigurable struct {
	TestString string
	Tool       ToolConfigurable `swap:"SubBox/Tool1"`
}

func (c *SubBoxConfigurable) Configure(configFiles ...string) error {
	return swap.Parse(c, configFiles...)
}

// ---------------------------------------------------------------------------------------------------------------------

func TestMixedBox(t *testing.T) {
	type Box struct {
		Tool                  ToolConfigurable
		PTRTool               *ToolConfigurable
		ToolNoConfigurable    Tool
		PTRToolNoConfigurable *Tool

		SubBox struct {
			Tool1 ToolMakeable     `swap:"SubBox/Tool1"`
			Tool2 *ToolMakeablePtr `swap:"SubBox/Tool2"`
			Tool3 *ToolMakeable    `swap:"SubBox/Tool3"`
			Tool4 ToolMakeablePtr  `swap:"SubBox/Tool4"`
		}

		ToolRegistered Tool2 `swap:"Tool"`

		SubBoxConfigurable SubBoxConfigurable `swap:"Tool"`

		ToolOmit    ToolConfigurable  `swap:"-"`
		PTRToolOmit *ToolConfigurable `swap:"-"`
	}

	defaultToolConfig := ToolConfig{TestString: "0"}
	createJSON(defaultToolConfig, "Tool.json", t)
	defaultToolConfig.TestString = "00"
	createTOML(defaultToolConfig, "PTRTool.toml", t)
	defaultToolConfig.TestString = "1"
	createYAML(defaultToolConfig, "SubBox/Tool1.yaml", t)
	defaultToolConfig.TestString = "2"
	createYAML(defaultToolConfig, "SubBox/Tool2.yaml", t)
	defaultToolConfig.TestString = "3"
	createYAML(defaultToolConfig, "SubBox/Tool3.yaml", t)
	defaultToolConfig.TestString = "4"
	createYAML(defaultToolConfig, "SubBox/Tool4.yaml", t)
	defer removeConfigFiles(t)

	swap.FileSearchCaseSensitive = true

	builder := swap.NewBuilder(configPath)
	builder.DebugOptions.Enabled = true
	//builder.DebugLevel = 3
	builder.DebugOptions.HideUnhandled = false
	builder.RegisterType(reflect.TypeOf(Tool2{}),
		func(configFiles ...string) (i interface{}, err error) {
			instance := &Tool2{}
			err = swap.Parse(&instance, configFiles...)
			return instance, err
		})

	fmt.Println(logger.Cyan(builder.EnvHandler.Sources.Git.Build))
	var test Box
	err := builder.Build(&test)

	require.Nil(t, err)
	require.Equal(t, "0", test.Tool.Config.TestString)
	require.Equal(t, "00", test.PTRTool.Config.TestString)
	require.Equal(t, 0, len(test.ToolNoConfigurable.TestString))
	require.Equal(t, 0, len(test.PTRToolNoConfigurable.TestString))
	require.Equal(t, "1", test.SubBox.Tool1.Config.TestString)
	require.Equal(t, "2", test.SubBox.Tool2.Config.TestString)
	require.Equal(t, "3", test.SubBox.Tool3.Config.TestString)
	require.Equal(t, "4", test.SubBox.Tool4.Config.TestString)
	require.Equal(t, "0", test.ToolRegistered.TestString)
	require.Equal(t, "1", test.SubBoxConfigurable.Tool.Config.TestString)
	require.Equal(t, 0, len(test.ToolOmit.Config.TestString))
	require.Nil(t, test.PTRToolOmit)
}

func TestFactoryFuncWrongTypeBox(t *testing.T) {
	type Box struct {
		Tool ToolMakeableWrongReturnType
	}

	defaultToolConfig := ToolConfig{TestString: "0"}
	createJSON(defaultToolConfig, "Tool.json", t)
	defer removeConfigFiles(t)

	var test Box
	builder := swap.NewBuilder(configPath)
	builder.DebugOptions.Enabled = true
	err := builder.Build(&test)
	require.NotNil(t, err)
}

func TestBoxNested(t *testing.T) {
	defaultToolConfig := ToolConfig{TestString: "0"}
	createJSON(defaultToolConfig, "Tool.json", t)
	defer removeConfigFiles(t)

	type BoxNested struct {
		Toola ToolConfigurable `swap:"Tool"`

		SubBoxa struct {
			Tool1a   ToolConfigurable `swap:"Tool"`
			SubBoxa2 struct {
				Tool1a2 ToolConfigurable `swap:"Tool"`
				Tool1a3 ToolConfigurable `swap:"Tool"`
			}
		}

		PTRTool *ToolConfigurable `swap:"Tool"`

		SubBoxb struct {
			Tool1b   ToolConfigurable `swap:"Tool"`
			SubBoxb2 struct {
				Tool1b2 ToolConfigurable `swap:"Tool"`
			}
		}

		Toolb ToolConfigurable `swap:"Tool"`
	}

	var test BoxNested
	builder := swap.NewBuilder(configPath)
	builder.DebugOptions.Enabled = true
	swap.SetColoredLogs(false)
	err := builder.Build(&test)
	require.Nil(t, err)
}

func TestBoxError(t *testing.T) {
	defaultToolConfig := ToolConfig{TestString: "0"}
	createYAML(defaultToolConfig, "ToolError.yaml", t)
	defer removeConfigFiles(t)

	type BoxError struct {
		ToolError ToolError
	}

	var test BoxError
	builder := swap.NewBuilder(configPath)
	err := builder.Build(&test)
	require.NotNil(t, err)
}

func TestPTRToolError(t *testing.T) {
	defaultToolConfig := ToolConfig{TestString: "0"}
	createYAML(defaultToolConfig, "PTRToolError.yml", t)
	defer removeConfigFiles(t)

	type PTRToolError struct {
		PTRToolError *ToolError
	}

	var test PTRToolError
	builder := swap.NewBuilder(configPath)
	err := builder.Build(&test)
	require.NotNil(t, err)
}

func TestInvalidPointer(t *testing.T) {
	builder := swap.NewBuilder(configPath)

	var test1 *string
	err := builder.Build(&test1)
	require.NotNil(t, err)

	type Box struct {
		Tool ToolConfigurable
	}

	var test2 *Box
	err = builder.Build(test2)
	require.NotNil(t, err)
}

func TestNilBox(t *testing.T) {
	swap.SetColoredLogs(false)

	defaultToolConfig := ToolConfig{TestString: "0"}
	createJSON(defaultToolConfig, "Tool1.json", t)
	createTOML(defaultToolConfig, "Tool2.toml", t)
	defer removeConfigFiles(t)

	type BoxNil struct {
		Tool1 ToolConfigurable
		Tool2 *ToolConfigurable
	}

	builder := swap.NewBuilder(configPath)

	var test1 BoxNil
	err := builder.Build(&test1)
	require.Nil(t, err)
	require.NotEqual(t, 0, len(test1.Tool1.Config.TestString))
	require.NotEqual(t, 0, len(test1.Tool2.Config.TestString))

	var test2 *BoxNil
	err = builder.Build(test2)
	require.NotNil(t, err)

	var test3 = &BoxNil{}
	err = builder.Build(test3)
	require.Nil(t, err)
	require.NotEqual(t, 0, len(test3.Tool1.Config.TestString))
	require.NotEqual(t, 0, len(test3.Tool2.Config.TestString))

	swap.SetColoredLogs(true)
}

func TestConfigFiles(t *testing.T) {
	type BoxConfigFiles struct {
		Tool1 ToolConfigurable
		Tool2 ToolConfigurable
		Tool3 *ToolConfigurable
	}

	var test1 BoxConfigFiles
	require.Error(t, swap.NewBuilder(configPath).Build(&test1))

	defaultToolConfig := ToolConfig{TestString: "0"}
	createYAML(defaultToolConfig, "Tool1.yml", t)
	createJSON(defaultToolConfig, "Tool3.json", t)
	createTOML(defaultToolConfig, "Tool2.toml", t)
	defer removeConfigFiles(t)

	var test2 BoxConfigFiles

	if err := swap.NewBuilder(configPath).Build(&test2); err != nil {
		t.Error(err)
	}
	require.NotEqual(t, 0, len(test2.Tool1.Config.TestString))
	require.NotEqual(t, 0, len(test2.Tool2.Config.TestString))
	require.NotEqual(t, 0, len(test2.Tool3.Config.TestString))
}

func TestBoxTags(t *testing.T) {
	builder := swap.NewBuilder(configPath)
	customEH := swap.NewEnvironmentHandler(swap.DefaultEnvs.Slice())
	builder = builder.WithCustomEnvHandler(customEH)
	builder.EnvHandler.SetCurrent("dev")
	builder.DebugOptions.HideSkipped = false
	defaultToolConfig := ToolConfig{TestString: "0"}
	devConfig := defaultToolConfig
	devpath := "dev"
	devConfig.TestString = devpath

	createYAML(devConfig, "Tool7.development.yml", t)
	createYAML(defaultToolConfig, "Tool1.yml", t)
	createYAML(defaultToolConfig, "test.yml", t)
	createJSON(devConfig, "tool8.development.json", t)
	createTOML(defaultToolConfig, "Tool2.toml", t)
	defer removeConfigFiles(t)

	type BoxTags struct {
		Tool1 ToolConfigurable
		Tool2 ToolConfigurable  `swap:"-"`
		Tool3 ToolConfigurable  `swap:"test.yml"`
		Tool5 ToolConfigurable  `swap:"-"`
		Tool6 *ToolConfigurable `swap:"-"`
		Tool7 *ToolConfigurable
		Tool8 *ToolConfigurable `swap:"tool8"`
	}

	var test BoxTags
	err := builder.Build(&test)
	require.Nil(t, err)
	require.Equal(t, defaultToolConfig.TestString, test.Tool1.Config.TestString)
	require.NotEqual(t, defaultToolConfig.TestString, test.Tool2.Config.TestString)
	require.Equal(t, defaultToolConfig.TestString, test.Tool3.Config.TestString)
	require.Equal(t, 0, len(test.Tool5.Config.TestString))
	require.Nil(t, test.Tool6)
	require.Equal(t, devpath, test.Tool7.Config.TestString)
	require.Equal(t, devpath, test.Tool8.Config.TestString)
}

func TestBoxAfterConfig(t *testing.T) {
	defaultToolConfig := ToolConfig{TestString: "0"}
	createYAML(defaultToolConfig, "Tool1.yml", t)
	defer removeConfigFiles(t)

	type BoxAfterConfig struct {
		Tool1 ToolConfigurable
		Tool2 ToolConfigurable
		Tool3 *ToolConfigurable
	}

	tString := "must remain the same"
	test := BoxAfterConfig{}
	test.Tool2 = ToolConfigurable{Config: ToolConfig{TestString: tString}}
	test.Tool3 = &ToolConfigurable{Config: ToolConfig{TestString: tString}}
	if err := swap.NewBuilder(configPath).Build(&test); err != nil {
		t.Error(err)
	}

	require.NotEqual(t, 0, len(test.Tool1.Config.TestString))
	require.Equal(t, tString, test.Tool2.Config.TestString)
	require.Equal(t, tString, test.Tool3.Config.TestString)
}
