package swap

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/oblq/swap/internal/logger"
)

// sff: Struct field flags.
const (
	// struct field tag key
	sftBuilderKey = "swap"

	// to skip a struct field
	sffBuilderSkip = "-"
)

// ---------------------------------------------------------------------------------------------------------------------

// FileSearchCaseSensitive determine config files search mode, false by default.
var FileSearchCaseSensitive bool

func SetColoredLogs(enabled bool) {
	logger.DisabelColors = !enabled
}

// Configurable interface ----------------------------------------------------------------------------------------------

// Configurable interface allow the configuration of fields
// which are automatically initialized to their zero value.
type Configurable interface {
	Configure(configFiles ...string) error
}

// Factory interface (factory) -----------------------------------------------------------------------------------------

// FactoryFunc is the factory method type.
type FactoryFunc func(configFiles ...string) (interface{}, error)

// Factory is the abstract factory interface.
type Factory interface {
	New(configFiles ...string) (interface{}, error)
}

// Implementation ------------------------------------------------------------------------------------------------------

type debugOptions struct {
	// Enabled true will print the loaded objects.
	Enabled bool
	//Levels         int
	HideUnhandled bool
	HideSkipped   bool
}

type Builder struct {
	typeFactories map[reflect.Type]FactoryFunc

	configPath string

	mutex sync.Mutex

	EnvHandler *EnvironmentHandler

	DebugOptions debugOptions
}

// NewBuilder return a builder,
// a custom EnvHandler can be provided later.
func NewBuilder(configsPath string) *Builder {
	return &Builder{
		typeFactories: make(map[reflect.Type]FactoryFunc),
		configPath:    configsPath,
		EnvHandler:    NewEnvironmentHandler(DefaultEnvs.Slice()),
		DebugOptions: debugOptions{
			true,
			true,
			true,
		},
	}
}

func (s *Builder) WithCustomEnvHandler(eh *EnvironmentHandler) *Builder {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.EnvHandler = eh
	return s
}

// RegisterType register a configurator func for a specific type and
// return the builder itself.
func (s *Builder) RegisterType(t reflect.Type, factory FactoryFunc) *Builder {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.typeFactories[t] = factory
	return s
}

// Build initialize and (eventually) configure the provided struct pointer
// looking for the config files in the provided configPath.
func (s *Builder) Build(toolBox interface{}) (err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	t := reflect.TypeOf(toolBox).Elem()
	v := reflect.ValueOf(toolBox).Elem()

	if t.Kind() != reflect.Struct {
		return errors.New("'toolBox' parameter should be a struct pointer")
	}

	// nil pointer
	if !v.CanSet() || !v.IsValid() {
		return errors.New("'toolBox' parameter should be a struct pointer")
	}

	debugLogs, err := s.build(nil, v, 0)
	fmt.Printf("\nSwap: %s\n", s.EnvHandler.Current().Info())
	if s.DebugOptions.Enabled {
		s.debug(t.Name(), debugLogs)
	}
	return err
}

// Struct fields scan --------------------------------------------------------------------------------------------------

// level is the parent grade to the initially passed field value
func (s *Builder) build(sf *reflect.StructField, fv reflect.Value, level int) (logs []string, err error) {
	switch fv.Kind() {
	case reflect.Ptr:
		if !fv.CanSet() {
			if !s.DebugOptions.HideSkipped {
				logs = append(logs, getLogString(sf, stateSkipped, nil, level, []string{}))
			}
			return logs, nil
		}

		if sf != nil {
			if tag, found := sf.Tag.Lookup(sftBuilderKey); found && tag == sffBuilderSkip {
				if !s.DebugOptions.HideSkipped {
					logs = append(logs, getLogString(sf, stateSkipped, nil, level, []string{}))
				}
				return logs, nil
			}

			if sf.Anonymous || !fv.CanSet() {
				if !s.DebugOptions.HideSkipped {
					logs = append(logs, getLogString(sf, stateSkipped, nil, level, []string{}))
				}
				return logs, nil
			}

			if !reflect.DeepEqual(fv.Interface(), reflect.Zero(fv.Type()).Interface()) {
				return []string{getLogString(sf, stateAlreadyConfigured, nil, level, []string{})}, nil
			}
		}

		fv.Set(reflect.New(fv.Type().Elem()))
		return s.build(sf, fv.Elem(), level)

	case reflect.Struct:
		var configEnvFiles []string
		var state state
		configEnvFiles, state, err = s.setField(sf, fv)
		if state == stateSkipped {
			if !s.DebugOptions.HideSkipped {
				logs = append(logs, getLogString(sf, state, nil, level, configEnvFiles))
			}
			return logs, err
		}
		if err != nil ||
			state == stateAlreadyConfigured ||
			state == stateMadeFromInterface || state == stateMadeFromRegisteredFactory {
			return []string{getLogString(sf, state, err, level, configEnvFiles)}, err
		}

		subLogs := make([]string, 0)

		// configure sub-fields first
		for i := 0; i < fv.NumField(); i++ {
			ssf := fv.Type().Field(i)
			sfv := fv.Field(i)
			//subPath := filepath.Join(configPath, sf.Name)
			sLogs, err := s.build(&ssf, sfv, level+1)
			subLogs = append(subLogs, sLogs...)
			if err != nil {
				logs = append(logs, subLogs...)
				return logs, err
			}
		}

		if state == stateRoot {
			logs = append(logs, subLogs...)
			return logs, nil
		}

		if configEnvFiles, err = s.configure(fv, configEnvFiles); err != nil {
			if err == errNotConfigurable {
				if len(subLogs) > 0 {
					logs = append(logs, getLogString(sf, stateTraversing, nil, level, configEnvFiles))
					logs = append(logs, subLogs...)
				} else if !s.DebugOptions.HideUnhandled { //if level <= s.DebugLevel &&
					logs = append(logs, getLogString(sf, stateUnhandled, nil, level, configEnvFiles))
				}
				return logs, nil
			}
			logs = append(logs, getLogString(sf, state, err, level, configEnvFiles))
			return
		}

		logs = append(logs, getLogString(sf, stateConfigured, nil, level, configEnvFiles))
		logs = append(logs, subLogs...)
		return

	default:
		_, _, err = s.setField(sf, fv)
		return
	}
}

// Basic struct field operations ---------------------------------------------------------------------------------------

// setField set the field value.
// It also extract struct field tags values, and config files.
// Return skip == true if:
// - !reflect.Indirect(fv).CanSet().
// - sf.Anonymous.
// - !fv.IsZero().
// - Have the skip `-` tag.
// - Implement the `Factory` interface.
// - A `factoryFunc` for the fv.Type() has been registered.
func (s *Builder) setField(sf *reflect.StructField, fv reflect.Value) (configEnvFiles []string, status state, err error) {
	// sf is nil for the root object
	if sf == nil {
		//fv.Set(reflect.New(fv.Type()).Elem())
		return []string{}, stateRoot, nil
	}

	if !reflect.Indirect(fv).CanSet() || sf.Anonymous {
		status = stateSkipped
		return
	}

	if !reflect.DeepEqual(fv.Interface(), reflect.Zero(fv.Type()).Interface()) {
		status = stateAlreadyConfigured
		return
	}

	configEnvFiles = []string{sf.Name}
	if s.parseTags(&configEnvFiles, sf) {
		status = stateSkipped
		return
	}

	getEnvFiles := func(cf []string) (files []string, err error) {
		for i, file := range cf {
			cf[i] = filepath.Join(s.configPath, file)
		}

		return appendEnvFiles(s.EnvHandler.Current(), cf)
	}

	if factory, haveFactory := fv.Addr().Interface().(Factory); haveFactory {

		configEnvFiles, err = getEnvFiles(configEnvFiles)
		if err != nil {
			return
		}
		var obj interface{}
		obj, err = factory.New(configEnvFiles...)
		if err != nil {
			return
		}
		got := reflect.ValueOf(obj)
		if reflect.Indirect(fv).Type() != reflect.Indirect(got).Type() {
			err = fmt.Errorf("wrong type returned from the Makeable interface for %s (%s): %s",
				sf.Name, sf.Type.String(), got.Type().String())
			return
		}
		indirect := reflect.Indirect(fv)
		indirect.Set(reflect.Indirect(got).Convert(indirect.Type()))
		status = stateMadeFromInterface

	} else if factory, haveRegisteredFactory := s.typeFactories[fv.Type()]; haveRegisteredFactory {

		configEnvFiles, err = getEnvFiles(configEnvFiles)
		if err != nil {
			return
		}
		var obj interface{}
		obj, err = factory(configEnvFiles...)
		if err != nil {
			return
		}
		got := reflect.ValueOf(obj)
		if reflect.Indirect(fv).Type() != reflect.Indirect(got).Type() {
			err = fmt.Errorf("wrong type returned from the registered factoryFunc for %s (%s): %s",
				sf.Name, sf.Type.String(), got.Type().String())
			return
		}
		indirect := reflect.Indirect(fv)
		indirect.Set(reflect.Indirect(got).Convert(indirect.Type()))
		status = stateMadeFromRegisteredFactory

	} else {

		fv.Set(reflect.New(fv.Type()).Elem())

	}

	return
}

// parseTags returns the config file name and the skip flag.
// The name will be returned also if not specified in tags,
// the field name without extension will be returned in that case,
// loadConfig will look for a file with that prefix and any kind
// of extension, if necessary (no '.' in file name).
func (s *Builder) parseTags(configFiles *[]string, f *reflect.StructField) (skip bool) {
	tag, found := f.Tag.Lookup(sftBuilderKey)
	if !found {
		return
	}

	if tag == sffBuilderSkip {
		return true
	}

	tagFields := strings.Split(tag, ",")
	for _, flag := range tagFields {
		files := strings.Split(flag, "|")
		*configFiles = append(*configFiles, files...)
	}

	return
}

// Struct fields config ------------------------------------------------------------------------------------------------

// configure will call the 'Configurable' interface on the passed field struct pointer.
func (s *Builder) configure(fv reflect.Value, configFiles []string) (configEnvFiles []string, err error) {
	if _, isConfigurable := fv.Addr().Interface().(Configurable); isConfigurable {
		for i, file := range configFiles {
			configFiles[i] = filepath.Join(s.configPath, file)
		}
		configEnvFiles, err = appendEnvFiles(s.EnvHandler.Current(), configFiles)
		if err != nil {
			return configEnvFiles, err
		}
		return configEnvFiles, fv.Addr().Interface().(Configurable).Configure(configEnvFiles...)
	} else {
		return configEnvFiles, errNotConfigurable
	}
}

func (s *Builder) debug(objName string, logs []string) {
	vcs := s.EnvHandler.Sources.Git.Info()
	fmt.Printf("%s\n", vcs)

	fmt.Println(logger.Magenta("type ") + logger.Yellow(objName) + logger.Magenta(" struct") + " {")
	for _, log := range logs {
		fmt.Print(log)
	}
	fmt.Print("}\n\n")
}

// Helpers -------------------------------------------------------------------------------------------------------------

var errNotConfigurable = errors.New("`Configurable` interface not implemented")

type state int

const (
	stateZero state = iota
	stateRoot
	stateSkipped
	stateAlreadyConfigured
	stateUnhandled
	stateTraversing
	stateConfigured
	stateMadeFromInterface
	stateMadeFromRegisteredFactory
)

func (s state) string() string {
	switch s {
	case stateZero:
		return ""
	case stateRoot:
		return "loading"
	case stateSkipped:
		return "skip"
	case stateAlreadyConfigured:
		return "already configured..."
	case stateUnhandled:
		return "unhandled..."
	case stateTraversing:
		return "traversing"
	case stateConfigured:
		return "configured"
	case stateMadeFromInterface:
		return "made with `Factory` interface"
	case stateMadeFromRegisteredFactory:
		return "made with registered `FactoryFunc`"
	default:
		return ""
	}
}

func getLogString(sf *reflect.StructField, state state, err error, level int, configFiles []string) string {
	objNameType := ""
	var t reflect.Type
	objType := " "

	if sf == nil {
		objNameType = "root"
	} else {
		objNameType = sf.Name
		t = sf.Type
		objType = t.String()
	}

	if len(objNameType) == 0 {
		if t != nil {
			objNameType = t.Name()
		} else {
			objNameType = "unknown"
		}
	}

	repetitions := int(math.Max(float64(level)-1, 0))
	if repetitions > 0 {
		objNameType = strings.Repeat("   ", repetitions) + "└─ " + objNameType
	} else {
		objNameType = "  " + objNameType
	}

	if len(objType)+len(objNameType)+1 >= 60 {
		if t != nil {
			objType = t.Kind().String()
		} else {
			objType = "unknown"
		}
	}

	//parts := strings.Split(objType, ".")
	//if len(parts) > 0 {
	//	if len(parts) > 1 {
	//		objType = parts[0]
	//		objType = strings.ReplaceAll(logger.Green(objType), "*", logger.Def("*"))
	//		objType += "." + logger.Yellow(parts[1])
	//	} else {
	//		objType = strings.ReplaceAll(logger.Yellow(parts[0]), "*", logger.Def("*"))
	//	}
	//}

	objNameType = fmt.Sprintf("%v %v", logger.Def(objNameType), logger.DarkGrey(objType))
	objNameType = fmt.Sprintf("%-80v", objNameType)

	if err != nil {
		switch err {
		default:
			return fmt.Sprintf("%s %s\n", objNameType, "-> "+logger.Red(err.Error()))
		}
	} else {
		inArrow := "<- "
		outArrow := "-> "

		switch state {
		case stateRoot:
			return fmt.Sprintf("%s %s\n", objNameType, inArrow+logger.Def(state.string()))

		case stateTraversing:
			return fmt.Sprintf("%s %s\n", objNameType, inArrow+logger.Def(state.string()))

		case stateSkipped:
			return fmt.Sprintf("%s %s\n", objNameType, outArrow+logger.Yellow(state.string()))

		case stateAlreadyConfigured:
			return fmt.Sprintf("%s %s\n", objNameType, outArrow+logger.White(state.string()))

		case stateUnhandled:
			return fmt.Sprintf("%s %s\n", objNameType, outArrow+logger.LightGrey(state.string()))

		case stateConfigured:
			for i, file := range configFiles {
				configFiles[i] = filepath.Base(file)
			}
			return fmt.Sprintf("%s %-49s <- (%s)\n",
				objNameType, inArrow+logger.Green(state.string()), logger.LightGrey(strings.Join(configFiles, ", ")))

		case stateMadeFromInterface, stateMadeFromRegisteredFactory:
			for i, file := range configFiles {
				configFiles[i] = filepath.Base(file)
			}
			return fmt.Sprintf("%s %-49s <- (%s)\n",
				objNameType, inArrow+logger.Blue(state.string()), logger.LightGrey(strings.Join(configFiles, ", ")))

		default:
			return fmt.Sprintf("%s %s\n", objNameType, inArrow+state.string())
		}
	}
}
