package logger

import (
	"fmt"
)

// DisableColors turn off colors code injections.
var DisableColors = false

type color string

// Color ANSI codes
const (
	whiteCol     color = "97m"
	defaultCol   color = "39m"
	lightGreyCol color = "37m"
	darkGreyCol  color = "90m"

	redCol     color = "31m"
	greenCol   color = "32m"
	yellowCol  color = "33m"
	blueCol    color = "34m"
	magentaCol color = "35m"
	cyanCol    color = "36m"

	esc   = "\033["
	clear = "\033[0m"
)

// Painter define a func that return a colored
// string representation of the passed argument.
type Painter func(interface{}) string

// White return the argument as a color escaped string
func White(arg interface{}) string {
	return colored(arg, whiteCol)
}

// Def return the argument as a color escaped string
func Def(arg interface{}) string {
	return colored(arg, defaultCol)
}

// LightGrey return the argument as a color escaped string
func LightGrey(arg interface{}) string {
	return colored(arg, lightGreyCol)
}

// DarkGrey return the argument as a color escaped string
func DarkGrey(arg interface{}) string {
	return colored(arg, darkGreyCol)
}

// Red return the argument as a color escaped string
func Red(arg interface{}) string {
	return colored(arg, redCol)
}

// Green return the argument as a color escaped string
func Green(arg interface{}) string {
	return colored(arg, greenCol)
}

// Cyan return the argument as a color escaped string
func Cyan(arg interface{}) string {
	return colored(arg, cyanCol)
}

// Yellow return the argument as a color escaped string
func Yellow(arg interface{}) string {
	return colored(arg, yellowCol)
}

// Blue return the argument as a color escaped string
func Blue(arg interface{}) string {
	return colored(arg, blueCol)
}

// Magenta return the argument as a color escaped string
func Magenta(arg interface{}) string {
	return colored(arg, magentaCol)
}

// colored return the ANSI colored formatted string.
func colored(arg interface{}, color color) string {
	argString := fmt.Sprint(arg)
	if len(argString) > 0 && len(color) > 0 && !DisableColors {
		return fmt.Sprintf("%s%s%s%s", esc, color, arg, clear)
	}
	return argString
}

// KVLogger is an ansi instance type for Key-Value logging.
type KVLogger struct {
	KeyPainter   Painter
	ValuePainter Painter
}

// Sprint return the key with predefined KeyColor and KeyMaxWidth and
// the value with the predefined ValueColor in string format.
func (kv *KVLogger) Sprint(key interface{}, value interface{}) string {
	k, v := kv.Ansify(key, value)
	return fmt.Sprintf("%s%s", k, v)
}

// Ansify return a colored string representation
// of the key-value couple.
func (kv *KVLogger) Ansify(key interface{}, value interface{}) (string, string) {
	var k, v string

	k = fmt.Sprintf("%-20v", key)

	if kv.KeyPainter == nil {
		kv.KeyPainter = Def
	}

	k = kv.KeyPainter(k)

	if kv.ValuePainter != nil {
		v = kv.ValuePainter(value)
	} else {
		v = fmt.Sprint(value)
	}

	return k, v
}
