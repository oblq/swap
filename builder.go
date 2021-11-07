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
	// struct field tag key, takes additional
	// config files path, relatives to the
	// initial one provided in on init.
	sftBuilderKey = "swap"

	// to skip a struct field
	sffBuilderSkip = "-"
)

// This is automatically set when the builder constructor is called.
var globalFS = NewFileSystemLocal(".")

// ---------------------------------------------------------------------------------------------------------------------

// SetColoredLogs enable / disable colors in the stdOut.
func SetColoredLogs(enabled bool) {
	logger.DisableColors = !enabled
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

// Builder recursively build/configure struct fields
// on the given struct, choosing the right configuration files
// based on the build environment.
type Builder struct {
	typeFactories map[reflect.Type]FactoryFunc

	//configPath string
	fileSystem FileSystem

	mutex sync.Mutex

	EnvHandler *EnvironmentHandler

	DebugOptions debugOptions
}

// NewBuilder return a builder,
// a custom EnvHandler can be provided later.
func NewBuilder(fileSystem FileSystem) *Builder {
	globalFS = fileSystem

	return &Builder{
		typeFactories: make(map[reflect.Type]FactoryFunc),
		fileSystem:    fileSystem,
		EnvHandler:    NewEnvironmentHandler(DefaultEnvs.Slice()),
		DebugOptions: debugOptions{
			true,
			true,
			true,
		},
	}
}

// WithCustomEnvHandler return the same instance of the Builder
// but with the custom environmentHandler.
func (b *Builder) WithCustomEnvHandler(eh *EnvironmentHandler) *Builder {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.EnvHandler = eh
	return b
}

// RegisterType register a configurator func for a specific type and
// return the builder itself.
func (b *Builder) RegisterType(t reflect.Type, factory FactoryFunc) *Builder {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	b.typeFactories[t] = factory
	return b
}

// Build initialize and (eventually) configure the provided struct pointer
// looking for the config files in the provided configPath.
func (b *Builder) Build(toolBox interface{}) (err error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	t := reflect.TypeOf(toolBox).Elem()
	v := reflect.ValueOf(toolBox).Elem()

	if t.Kind() != reflect.Struct {
		return errors.New("'toolBox' parameter should be a struct pointer")
	}

	// nil pointer
	if !v.CanSet() || !v.IsValid() {
		return errors.New("'toolBox' parameter should be a struct pointer")
	}

	debugLogs, err := b.build(nil, v, 0)
	if b.DebugOptions.Enabled {
		fmt.Printf("\nSwap: %s\n", b.EnvHandler.Current().Info())
		b.debug(t.Name(), debugLogs)
	}
	return err
}

// Struct fields scan --------------------------------------------------------------------------------------------------

// level is the parent grade to the initially passed field value
func (b *Builder) build(sf *reflect.StructField, fv reflect.Value, level int) (logs []string, err error) {
	switch fv.Kind() {
	case reflect.Ptr:
		if !fv.CanSet() {
			if !b.DebugOptions.HideSkipped {
				logs = append(logs, getLogString(sf, stateSkipped, nil, level, []string{}))
			}
			return logs, nil
		}

		if sf != nil {
			if tag, found := sf.Tag.Lookup(sftBuilderKey); found && tag == sffBuilderSkip {
				if !b.DebugOptions.HideSkipped {
					logs = append(logs, getLogString(sf, stateSkipped, nil, level, []string{}))
				}
				return logs, nil
			}

			if sf.Anonymous || !fv.CanSet() {
				if !b.DebugOptions.HideSkipped {
					logs = append(logs, getLogString(sf, stateSkipped, nil, level, []string{}))
				}
				return logs, nil
			}

			if !reflect.DeepEqual(fv.Interface(), reflect.Zero(fv.Type()).Interface()) {
				return []string{getLogString(sf, stateAlreadyConfigured, nil, level, []string{})}, nil
			}
		}

		fv.Set(reflect.New(fv.Type().Elem()))
		return b.build(sf, fv.Elem(), level)

	case reflect.Struct:
		var configEnvFiles []string
		var state state
		configEnvFiles, state, err = b.setField(sf, fv)
		if state == stateSkipped {
			if !b.DebugOptions.HideSkipped {
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
			sLogs, err := b.build(&ssf, sfv, level+1)
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

		if configEnvFiles, err = b.configure(fv, configEnvFiles); err != nil {
			if err == errNotConfigurable {
				if len(subLogs) > 0 {
					logs = append(logs, getLogString(sf, stateTraversing, nil, level, configEnvFiles))
					logs = append(logs, subLogs...)
				} else if !b.DebugOptions.HideUnhandled { //if level <= b.DebugLevel &&
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
		_, _, err = b.setField(sf, fv)
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
func (b *Builder) setField(sf *reflect.StructField, fv reflect.Value) (configEnvFiles []string, status state, err error) {
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
	if b.parseTags(&configEnvFiles, sf) {
		status = stateSkipped
		return
	}

	if factory, haveFactory := fv.Addr().Interface().(Factory); haveFactory {

		configEnvFiles, err = getConfigPathsByFieldTagFileNames(configEnvFiles, b.fileSystem, b.EnvHandler.Current())
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

	} else if factory, haveRegisteredFactory := b.typeFactories[fv.Type()]; haveRegisteredFactory {

		configEnvFiles, err = getConfigPathsByFieldTagFileNames(configEnvFiles, b.fileSystem, b.EnvHandler.Current())
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
func (b *Builder) parseTags(configFiles *[]string, f *reflect.StructField) (skip bool) {
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
func (b *Builder) configure(fv reflect.Value, configFiles []string) (configEnvFiles []string, err error) {
	if _, isConfigurable := fv.Addr().Interface().(Configurable); isConfigurable {
		configEnvFiles, err = getConfigPathsByFieldTagFileNames(configFiles, b.fileSystem, b.EnvHandler.Current())
		if err != nil {
			return configEnvFiles, err
		}
		return configEnvFiles, fv.Addr().Interface().(Configurable).Configure(configEnvFiles...)
	}

	return configEnvFiles, errNotConfigurable
}

func (b *Builder) debug(objName string, logs []string) {
	vcs := b.EnvHandler.Sources.Git.Info()
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
			return fmt.Sprintf("%s %-46s <- (%s)\n",
				objNameType, inArrow+logger.Green(state.string()), logger.LightGrey(strings.Join(configFiles, ", ")))

		case stateMadeFromInterface, stateMadeFromRegisteredFactory:
			for i, file := range configFiles {
				configFiles[i] = filepath.Base(file)
			}
			return fmt.Sprintf("%s %-46s <- (%s)\n",
				objNameType, inArrow+logger.Blue(state.string()), logger.LightGrey(strings.Join(configFiles, ", ")))

		default:
			return fmt.Sprintf("%s %s\n", objNameType, inArrow+state.string())
		}
	}
}
