// Package swap is an agnostic config parser
// (supporting YAML, TOML, JSON and environment vars) and
// a toolbox factory with automatic configuration
// based on your build environment.
package swap

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// todo: replace encoding/json with github.com/json-iterator/go

const (
	// struct field tag key
	sftConfigKey = "swapcp"

	// return error if missing value
	// eg.: `swap:"required"`
	sffConfigRequired = "required"

	// sffEnv environment var value can be in json format,
	// it also overrides the default value.
	// eg.: `swap:"env=env_var_name"`
	sffConfigEnv = "env"

	// set the default value
	// eg.: `swap:"default=1"`
	sffConfigDefault = "default"
)

var (
	// files type regexp
	regexpValidExt = regexp.MustCompile(`(?i)(.y(|a)ml|.toml|.json)`)
	regexpYAML     = regexp.MustCompile(`(?i)(.y(|a)ml)`)
	regexpTOML     = regexp.MustCompile(`(?i)(.toml)`)
	regexpJSON     = regexp.MustCompile(`(?i)(.json)`)
)

// Parse strictly parse only the specified config files
// in the exact order they are into the config interface, one by one.
// The latest files will override the former.
// Will also parse fmt template keys in configs and struct flags.
func Parse(config interface{}, files ...string) (err error) {
	return ParseByEnv(config, nil, files...)
}

// ParseByEnv parse all the passed files plus all the matched ones
// for the given Environment (if not nil) into the config interface.
// Environment specific files will override generic files.
// The latest files passed will override the former.
// Will also parse fmt template keys and struct flags.
func ParseByEnv(config interface{}, env *Environment, files ...string) (err error) {
	files, err = appendEnvFiles(env, files)
	if err != nil {
		return fmt.Errorf("no config file found for '%s': %s", strings.Join(files, " | "), err.Error())
	}

	if len(files) == 0 {
		return fmt.Errorf("no config file found for '%s'", strings.Join(files, " | "))
	}

	if reflect.TypeOf(config).Kind() != reflect.Ptr {
		return fmt.Errorf("the config argument should be a pointer: `%s`", reflect.TypeOf(config).String())
	}

	for _, file := range files {
		if err = unmarshalFile(file, config); err != nil {
			return err
		}
		if err = parseTemplateFile(file, config); err != nil {
			return err
		}
	}

	return parseConfigTags(config)
}

// File search ---------------------------------------------------------------------------------------------------------

// appendEnvFiles will search for the given file names in the given path
// returning all the eligible files (eg.: <path>/config.yaml or <path>/config.<environment>.json)
//
// Files name can also be passed without file extension,
// configFilesByEnv is semi-agnostic and will match any
// supported extension using the regex: `(?i)(.y(|a)ml|.toml|.json)`.
//
// The 'file' name will be searched as (in that order):
//  - '<path>/<file>(.* || <the_provided_extension>)'
//  - '<path>/<file>.<environment>(.* || <the_provided_extension>)'
//
// The latest found files will override previous.
func appendEnvFiles(env *Environment, files []string) (foundFiles []string, err error) {
	for _, file := range files {
		configPath, fileName := filepath.Split(file)
		if len(configPath) == 0 {
			configPath = "./"
		}

		ext := filepath.Ext(fileName)
		extTrimmed := strings.TrimSuffix(fileName, ext)
		if len(ext) == 0 {
			ext = regexpValidExt.String() // search for any compatible file
		}

		format := "^%s%s$"
		if !FileSearchCaseSensitive {
			format = "(?i)(^%s)%s$"
		}
		// look for the config file in the config path (eg.: tool.yml)
		regex := regexp.MustCompile(fmt.Sprintf(format, extTrimmed, ext))
		var foundFile string
		foundFile, err = walkConfigPath(configPath, regex)
		if err != nil {
			break
		}
		if len(foundFile) > 0 {
			foundFiles = append(foundFiles, foundFile)
		}

		if env != nil {
			// look for the env config file in the config path (eg.: tool.development.yml)
			//regexEnv := regexp.MustCompile(fmt.Sprintf(format, fmt.Sprintf("%s.%s", extTrimmed, Env().ID()), ext))
			regexEnv := regexp.MustCompile(fmt.Sprintf(format, fmt.Sprintf("%s.%s", extTrimmed, env.Tag()), ext))
			foundFile, err = walkConfigPath(configPath, regexEnv)
			if err != nil {
				break
			}
			if len(foundFile) > 0 {
				foundFiles = append(foundFiles, foundFile)
			}
		}
	}

	if err == nil && len(foundFiles) == 0 {
		err = fmt.Errorf("no config file found for '%s'", strings.Join(files, " | "))
	}
	return
}

// walkConfigPath look for a file matching the passed regex skipping sub-directories.
func walkConfigPath(configPath string, regex *regexp.Regexp) (matchedFile string, err error) {
	err = filepath.Walk(configPath, func(path string, info os.FileInfo, err error) error {
		// nil if the path does not exist
		if info == nil {
			return filepath.SkipDir
		}

		if info.IsDir() && info.Name() != filepath.Base(configPath) {
			return filepath.SkipDir
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if regex.MatchString(info.Name()) {
			matchedFile = path
		}

		return nil
	})

	return
}

// File parse ----------------------------------------------------------------------------------------------------------

func unmarshalFile(file string, config interface{}) (err error) {
	var in []byte
	if in, err = ioutil.ReadFile(file); err != nil {
		return err
	}
	ext := filepath.Ext(file)

	switch {
	case regexpYAML.MatchString(ext):
		err = unmarshalYAML(in, config)
	case regexpTOML.MatchString(ext):
		err = unmarshalTOML(in, config)
	case regexpJSON.MatchString(ext):
		err = unmarshalJSON(in, config)
	default:
		err = fmt.Errorf("unknown data format, can't unmarshal file: '%s'", file)
	}

	return
}

func unmarshalJSON(data []byte, config interface{}) (err error) {
	return json.Unmarshal(data, config)
}

func unmarshalTOML(data []byte, config interface{}) (err error) {
	_, err = toml.Decode(string(data), config)
	return err
}

func unmarshalYAML(data []byte, config interface{}) (err error) {
	return yaml.Unmarshal(data, config)
}

// parseTemplateFile parse all text/template placeholders
// (eg.: {{.Key}}) in config files.
func parseTemplateFile(file string, config interface{}) error {
	tpl, err := template.ParseFiles(file)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err = tpl.Execute(&buf, config); err != nil {
		return err
	}

	ext := filepath.Ext(file)

	switch {
	case regexpYAML.MatchString(ext):
		return unmarshalYAML(buf.Bytes(), config)
	case regexpTOML.MatchString(ext):
		return unmarshalTOML(buf.Bytes(), config)
	case regexpJSON.MatchString(ext):
		return unmarshalJSON(buf.Bytes(), config)
	default:
		return fmt.Errorf("unknown data format, can't unmarshal file: '%s'", file)
	}
}

// Flags parse ---------------------------------------------------------------------------------------------------------

// parseConfigTags will process the struct field tags.
func parseConfigTags(elem interface{}) error {
	elemValue := reflect.Indirect(reflect.ValueOf(elem))

	switch elemValue.Kind() {

	case reflect.Struct:
		elemType := elemValue.Type()
		//fmt.Printf("%sProcessing STRUCT: %s = %+v\n", indent, elemType.Name(), elem)

		for i := 0; i < elemType.NumField(); i++ {

			ft := elemType.Field(i)
			fv := elemValue.Field(i)

			if !fv.CanAddr() || !fv.CanInterface() {
				//fmt.Printf("%sCan't addr or interface FIELD: CanAddr: %v, CanInterface: %v. -> %s = '%+v'\n", indent, fv.CanAddr(), fv.CanInterface(), ft.Name, fv.Interface())
				continue
			}

			tag := ft.Tag.Get(sftConfigKey)
			tagFields := strings.Split(tag, ",")
			//fmt.Printf("\n%sProcessing FIELD: %s %s = %+v, tags: %s\n", indent, ft.Name, ft.Type.String(), fv.Interface(), tag)
			for _, flag := range tagFields {

				kv := strings.Split(flag, "=")

				if kv[0] == sffConfigEnv {
					if len(kv) == 2 {
						if value := os.Getenv(kv[1]); len(value) > 0 {
							//debugPrintf("Loading configuration for struct `%v`'s field `%v` from env %v...\n", elemType.Name(), ft.Name, kv[1])
							if err := yaml.Unmarshal([]byte(value), fv.Addr().Interface()); err != nil {
								return err
							}
						}
					} else {
						return fmt.Errorf("missing environment variable key value in tag: %s, must be someting like: `%s:\"env=env_var_name\"`",
							sftConfigKey, flag)
					}
				}

				if empty := reflect.DeepEqual(fv.Interface(), reflect.Zero(fv.Type()).Interface()); empty {
					if kv[0] == sffConfigDefault {
						if len(kv) == 2 {
							if err := yaml.Unmarshal([]byte(kv[1]), fv.Addr().Interface()); err != nil {
								return err
							}
						} else {
							return fmt.Errorf("missing default value in tag: %s, must be someting like: `%s:\"default=true\"`",
								sftConfigKey, flag)
						}
					} else if kv[0] == sffConfigRequired {
						return errors.New(ft.Name + " is required")
					}
				}
			}

			switch fv.Kind() {
			case reflect.Ptr, reflect.Struct, reflect.Slice, reflect.Map:
				if err := parseConfigTags(fv.Addr().Interface()); err != nil {
					return err
				}
			}

			//fmt.Printf("%sProcessed  FIELD: %s %s = %+v\n", indent, ft.Name, ft.Type.String(), fv.Interface())
		}

	case reflect.Slice:
		for i := 0; i < elemValue.Len(); i++ {
			if err := parseConfigTags(elemValue.Index(i).Addr().Interface()); err != nil {
				return err
			}
		}

	case reflect.Map:
		for _, key := range elemValue.MapKeys() {
			if err := parseConfigTags(elemValue.MapIndex(key).Interface()); err != nil {
				return err
			}
		}
	}

	return nil
}
