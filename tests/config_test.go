package tests

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/oblq/swap"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Helpers -------------------------------------------------------------------------------------------------------------

const configPath = "/tmp/swap"

func createYAML(object interface{}, fileName string, t *testing.T) {
	confBytes, err := yaml.Marshal(object)
	if err != nil {
		t.Errorf("failed to create config file: %v", err)
	}
	writeFiles(fileName, confBytes, t)
}

func createTOML(object interface{}, fileName string, t *testing.T) {
	var buffer bytes.Buffer
	if err := toml.NewEncoder(&buffer).Encode(object); err != nil {
		t.Errorf("failed to create config file: %v", err)
	}
	writeFiles(fileName, buffer.Bytes(), t)
}

func createJSON(object interface{}, fileName string, t *testing.T) {
	confBytes, err := json.Marshal(object)
	if err != nil {
		t.Errorf("failed to create config file: %v", err)
	}
	writeFiles(fileName, confBytes, t)
}

func writeFiles(fileName string, bytes []byte, t *testing.T) {
	filePath := filepath.Join(configPath, fileName)

	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		t.Error(err)
	}

	if err := ioutil.WriteFile(filePath, bytes, os.ModePerm); err != nil {
		t.Errorf("failed to create config file: %v", err)
	}
}

func removeConfigFiles(t *testing.T) {
	if err := os.RemoveAll(configPath); err != nil {
		t.Error(err)
	}
}

type Postgres struct {
	DB       string `swapcp:"env=POSTGRES_DB,default=postgres"`
	User     string `swapcp:"env=POSTGRES_USER,default=postgres"`
	Password string `swapcp:"env=POSTGRES_PASSWORD,required"`
	Port     int    `swapcp:"default=5432"`
}

type EmbeddedStruct struct {
	Field1 string `swapcp:"default=swap"`
	Field2 string `swapcp:"required"`
}

type TestConfig struct {
	String        string `swapcp:"default=swap"`
	PG            Postgres
	Slice         []string
	Map           *map[string]string
	EmbeddedSlice []EmbeddedStruct
	// EmbeddedStruct without pointer inside of a map would not be addressable,
	// so, this is the way that make sense...
	// Otherwise also 'config.EmbeddedMap["test"].Field1 = "a value"' can't be done.
	EmbeddedMap map[string]*EmbeddedStruct
}

func defaultConfig() TestConfig {
	config := TestConfig{
		String: "swap",
		Slice:  []string{"elem1", "elem2"},
		Map:    &map[string]string{"key": "value"},
		PG: Postgres{
			DB:       "swap",
			User:     "me",
			Password: "myPass123",
			Port:     5432,
		},
		EmbeddedSlice: []EmbeddedStruct{
			{
				Field1: "swap",
				Field2: "f2",
			},
		},
		EmbeddedMap: map[string]*EmbeddedStruct{
			"test": {
				Field1: "swap",
				Field2: "f2map",
			},
		},
	}
	return config
}

type ConfigWTemplates struct {
	Text1     string
	Text2     string
	TextSlice []string
	TextMap   map[string]string
	TStruct   struct {
		Text     string
		TStruct2 struct {
			Text string
		}
	}
}

func defaultConfigWTemplates() ConfigWTemplates {
	return ConfigWTemplates{
		Text1:     "Hello",
		Text2:     "{{.Text1}} world!",
		TextSlice: []string{"{{.Text1}} world!"},
		TextMap: map[string]string{
			"text": "{{.Text1}} world!",
		},
		TStruct: struct {
			Text     string
			TStruct2 struct{ Text string }
		}{
			Text: "{{.Text1}} world!",
			TStruct2: struct {
				Text string
			}{
				Text: "{{.Text1}} world!",
			},
		},
	}
}

// Tests ---------------------------------------------------------------------------------------------------------------

func TestYAML(t *testing.T) {
	config := defaultConfig()
	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result1 TestConfig
	err := swap.Parse(&result1, filepath.Join(configPath, fileName))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result1, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result1)

	var result2 TestConfig
	err = swap.Parse(&result2, filepath.Join(configPath, "config"))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result2, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result2)
}

func TestYML(t *testing.T) {
	config := defaultConfig()
	fileName := "config.yml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result1 TestConfig
	err := swap.Parse(&result1, filepath.Join(configPath, fileName))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result1, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result1)

	var result2 TestConfig
	err = swap.Parse(&result2, filepath.Join(configPath, "config"))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result2, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result2)
}

func TestTOML(t *testing.T) {
	config := defaultConfig()
	fileName := "config.toml"
	createTOML(config, fileName, t)
	defer removeConfigFiles(t)

	var result1 TestConfig
	err := swap.Parse(&result1, filepath.Join(configPath, fileName))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result1, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result1)

	var result2 TestConfig
	err = swap.Parse(&result2, filepath.Join(configPath, "config"))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result2, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result2)
}

func TestJSON(t *testing.T) {
	config := defaultConfig()
	fileName := "config.json"
	createJSON(config, fileName, t)
	defer removeConfigFiles(t)

	var result1 TestConfig
	err := swap.Parse(&result1, filepath.Join(configPath, fileName))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result1, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result1)

	var result2 TestConfig
	err = swap.Parse(&result2, filepath.Join(configPath, "config"))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result2, config), "\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", config, result2)
}

func TestParsingIntoNonStruct(t *testing.T) {
	config := defaultConfig()
	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result1 string
	err := swap.Parse(&result1, filepath.Join(configPath, fileName))
	require.NotNil(t, err, "LoadConfig should return an error")
}

func TestYAMLWrongPath(t *testing.T) {
	fileName := "config.yaml"
	var result1 TestConfig
	err := swap.Parse(&result1, fileName) // only passing filename
	require.NotNil(t, err, "LoadConfig should return an error")
}

func TestCorruptedFile(t *testing.T) {
	fileName := "config.yaml"
	createYAML("wrongObject", fileName, t)
	defer removeConfigFiles(t)

	var result TestConfig
	err := swap.Parse(&result, filepath.Join(configPath, fileName))
	require.NotNil(t, err, "corrupted file does not return error")
}

func TestWrongConfigFileName(t *testing.T) {
	config := defaultConfig()
	fileName := "config.wrong"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result TestConfig
	err := swap.Parse(&result, filepath.Join(configPath, fileName))
	require.NotNil(t, err, "wrong path does not return error")
}

func TestNotAStruct(t *testing.T) {
	config := defaultConfig()
	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result string
	err := swap.Parse(&result, filepath.Join(configPath, fileName))
	require.NotNil(t, err)
}

func TestNoFileName(t *testing.T) {
	config := defaultConfig()
	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result1 TestConfig
	err := swap.Parse(&result1, configPath)
	require.NotNil(t, err)
}

func TestMapYAML(t *testing.T) {
	config := defaultConfig()
	createYAML(config, "config1.yaml", t)
	config.String = "overriden1"
	createYAML(config, "config2.yaml", t)
	config.PG.DB = "overriden2"
	createYAML(config, "config3.yaml", t)
	defer removeConfigFiles(t)

	var configMap map[string]interface{}
	err := swap.Parse(&configMap,
		filepath.Join(configPath, "config1.yaml"),
		filepath.Join(configPath, "config2.yaml"),
		filepath.Join(configPath, "config3.yaml"),
	)
	require.Nil(t, err)
	require.Equal(t, "overriden1", configMap["string"])
	require.Equal(t, "overriden2", configMap["pg"].(map[string]interface{})["db"])
}

func TestMapJSON(t *testing.T) {
	config := defaultConfig()
	createJSON(config, "config1.json", t)
	config.String = "overridden1"
	createJSON(config, "config2.json", t)
	config.PG.DB = "overridden2"
	createJSON(config, "config3.json", t)
	defer removeConfigFiles(t)

	var configMap map[string]interface{}
	err := swap.Parse(&configMap,
		filepath.Join(configPath, "config1.json"),
		filepath.Join(configPath, "config2.json"),
		filepath.Join(configPath, "config3.json"),
	)

	require.Nil(t, err)
	require.Equal(t, "overridden1", configMap["String"])
	require.Equal(t, "overridden2", configMap["PG"].(map[string]interface{})["DB"])
}

func TestMapTOML(t *testing.T) {
	config := defaultConfig()
	createTOML(config, "config1.toml", t)
	config.String = "overridden1"
	createTOML(config, "config2.toml", t)
	config.PG.DB = "overridden2"
	createTOML(config, "config3.toml", t)
	defer removeConfigFiles(t)

	var configMap map[string]interface{}
	err := swap.Parse(&configMap,
		filepath.Join(configPath, "config1.toml"),
		filepath.Join(configPath, "config2.toml"),
		filepath.Join(configPath, "config3.toml"),
	)

	require.Nil(t, err)
	require.Equal(t, "overridden1", configMap["String"])
	require.Equal(t, "overridden2", configMap["PG"].(map[string]interface{})["DB"])
}

func TestMapMixed(t *testing.T) {
	config := defaultConfig()
	config.PG.DB = "overriddenyml"
	createYAML(config, "config1.yml", t)
	config.String = "overridden1"
	createTOML(config, "config2.toml", t)
	config.PG.DB = "overridden2"
	createJSON(config, "config3.json", t)
	defer removeConfigFiles(t)

	var configMap map[string]interface{}
	err := swap.Parse(&configMap,
		filepath.Join(configPath, "config1.yml"),
		filepath.Join(configPath, "config2.toml"),
		filepath.Join(configPath, "config3.json"),
	)

	require.Nil(t, err)
	require.Equal(t, "swap", configMap["string"])
	require.Equal(t, "overriddenyml", configMap["pg"].(map[string]interface{})["db"])
	require.Equal(t, "overridden1", configMap["String"])
	require.Equal(t, "overridden2", configMap["PG"].(map[string]interface{})["DB"])

	var configStruct TestConfig
	err = swap.Parse(&configStruct,
		filepath.Join(configPath, "config1.yml"),
		filepath.Join(configPath, "config2.toml"),
		filepath.Join(configPath, "config3.json"),
	)

	require.Nil(t, err)
	require.Equal(t, "overridden1", configStruct.String)
	require.Equal(t, "overridden2", configStruct.PG.DB)
}

func TestMapNoFiles(t *testing.T) {
	var configMap map[string]interface{}
	err := swap.Parse(configMap, filepath.Join(configPath, "config.yml"))
	require.NotNil(t, err)
}

//func TestUnmarshal(t *testing.T) {
//	defaultConfig := defaultConfig()
//	var configUnmarshal TestConfig
//
//	var tomlMarsh bytes.Buffer
//	err := toml.NewEncoder(&tomlMarsh).Encode(defaultConfig)
//	require.Nil(t, err)
//	err = swap.ConfigParser.Unmarshal(tomlMarsh.Bytes(), &configUnmarshal)
//	require.Nil(t, err)
//
//	confBytes, err := json.Marshal(defaultConfig)
//	require.Nil(t, err)
//	err = swap.ConfigParser.Unmarshal(confBytes, &configUnmarshal)
//	require.Nil(t, err)
//
//	confBytes, err = yaml.Marshal(defaultConfig)
//	require.Nil(t, err)
//	err = swap.ConfigParser.Unmarshal(confBytes, &configUnmarshal)
//	require.Nil(t, err)
//
//	// wrong bytes
//	err = swap.ConfigParser.Unmarshal([]byte("wrong"), &configUnmarshal)
//	require.NotNil(t, err)
//}

func TestConfigWTemplates(t *testing.T) {
	config := defaultConfigWTemplates()
	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result ConfigWTemplates
	err := swap.Parse(&result, filepath.Join(configPath, fileName))
	require.Nil(t, err)

	expected := "Hello world!"
	require.Equal(t, expected, result.Text2, "error in template parsing: %+v", result.Text2)
	require.Equal(t, expected, result.TextSlice[0], "error in template parsing: %+v", result.TextSlice[0])
	require.Equal(t, expected, result.TextMap["text"], "error in template parsing: %+v", result.TextMap["text"])
	require.Equal(t, expected, result.TStruct.Text, "error in template parsing: %+v", result.TStruct.Text)
	require.Equal(t, expected, result.TStruct.TStruct2.Text, "error in template parsing: %+v", result.TStruct.TStruct2.Text)

	//var uResult ConfigWTemplates
	//
	//confBytes, err := yaml.Marshal(config)
	//require.Nil(t, err)
	//err = swap.ConfigParser.Unmarshal(confBytes, &uResult)
	//require.Nil(t, err)
	//
	//require.Equal(t, expected, uResult.Text2, "error in template parsing: %+v", uResult.Text2)
	//require.Equal(t, expected, uResult.TextSlice[0], "error in template parsing: %+v", uResult.TextSlice[0])
	//require.Equal(t, expected, uResult.TextMap["text"], "error in template parsing: %+v", uResult.TextMap["text"])
	//require.Equal(t, expected, uResult.TStruct.Text, "error in template parsing: %+v", uResult.TStruct.Text)
	//require.Equal(t, expected, uResult.TStruct.TStruct2.Text, "error in template parsing: %+v", uResult.TStruct.TStruct2.Text)
}

// SFT = struct field tags
func TestSFTDefault(t *testing.T) {
	config := defaultConfig()
	config.String = ""
	config.PG.Port = 0
	config.EmbeddedSlice[0].Field1 = ""
	config.EmbeddedMap["test"].Field1 = ""

	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result TestConfig
	err := swap.Parse(&result, filepath.Join(configPath, fileName))
	require.Nil(t, err)
	require.True(t, reflect.DeepEqual(result, defaultConfig()),
		"\n\nFile:\n%#v\n\nConfig:\n%#v\n\n", defaultConfig(), result)
}

// SFT = struct field tags
func TestSFTRequired(t *testing.T) {
	config := defaultConfig()
	config.PG.Password = ""

	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	var result TestConfig
	err := swap.Parse(&result, filepath.Join(configPath, fileName))
	require.NotNil(t, err, "should return error if a required field is missing ")
}

// SFT = struct field tags
func TestSFTEnv(t *testing.T) {
	config := defaultConfig()
	config.PG.DB = "wrong"
	fileName := "config.yaml"
	createYAML(config, fileName, t)
	defer removeConfigFiles(t)

	err := os.Setenv("POSTGRES_DB", "postgres")
	require.Nil(t, err)

	var result TestConfig
	err = swap.Parse(&result, filepath.Join(configPath, fileName))
	require.Nil(t, err)
	require.Equal(t, result.PG.DB, "postgres", "env var not loaded correctly")

	if err := os.Unsetenv("POSTGRES_DB"); err != nil {
		t.Error(err)
	}
}

//func TestEnvironmentFiles(t *testing.T) {
//	eh := swap.NewEnvironmentHandler()
//	env := eh.Development
//
//	config := ToolConfigurable{}
//	createYAML(config, "tool1.yml", t)
//	//createJSON(config, "tool."+Env().ID()+".json", t)
//	createJSON(config, "tool."+env.Tag+".json", t)
//	createTOML(config, "tool.toml", t)
//	defer removeConfigFiles(t)
//
//	// '<path>/<file>.<environment>.*'
//	files1, err1 := swap.appendEnvFiles(env, filepath.Join(configPath, "tool"))
//	require.Nil(t, err1)
//	require.Equal(t, 2, len(files1))
//	require.Equal(t, filepath.Join(configPath, "tool."+env.Tag+".json"), files1[1])
//
//	// '<path>/<file>.*'
//	files2, err2 := swap.appendEnvFiles(env, filepath.Join(configPath, "tool1"))
//	require.Nil(t, err2)
//	require.Equal(t, 1, len(files2))
//	require.Equal(t, filepath.Join(configPath, "tool1.yml"), files2[0])
//
//	// '<path>/<file>.<ext>'
//	files3, err3 := swap.appendEnvFiles(env, filepath.Join(configPath, "tool.toml"))
//	require.Nil(t, err3)
//	require.Equal(t, 1, len(files3))
//	require.Equal(t, filepath.Join(configPath, "tool.toml"), files3[0])
//
//	// wrong name '<path>/<file>.<ext>'
//	_, err4 := swap.appendEnvFiles(env, filepath.Join(configPath, "tool2.toml"))
//	require.NotNil(t, err4)
//
//	// case insensitive '<path>/<file>.<environment>.*'
//	swap.FileSearchCaseSensitive = false
//	files5, err5 := swap.appendEnvFiles(env, filepath.Join(configPath, "TOOL"))
//	require.Nil(t, err5)
//	require.Equal(t, 2, len(files5))
//	require.Equal(t, filepath.Join(configPath, "tool."+env.Tag+".json"), files5[1])
//}
