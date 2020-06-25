package log

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type (
	MsgType uint8
	OutType uint8
)

const (
	MessageLog         MsgType = 0
	Message2Log        MsgType = 1
	WarningLog         MsgType = 2
	DebugLog           MsgType = 3
	ErrorLog           MsgType = 4
	FormattedOut       OutType = 0
	LineOut            OutType = 1
	DefaultMaxLineSize int     = 2000
	DefaultTimeFormat  string  = "2006/01/02 15:04:05"
)

// AdapterFunc is the type for the function adapter
// any function that has this signature can be used as an adapter
type AdapterFunc func(m MsgType, o OutType, config map[string]interface{}, msg ...interface{})

// AdapterPod contains the metadata of an adapter
type AdapterPod struct {
	Adapter AdapterFunc
	Config  map[string]interface{}
}

var (
	// DebugMode Enable debug mode
	DebugMode bool

	// EnableANSIColors enables ANSI colors, default true
	EnableANSIColors = true

	// MaxLineSize limits the size of the line, if the size
	// exceeds that indicated by MaxLineSize the system cuts
	// the string and adds "..." at the end.
	MaxLineSize = DefaultMaxLineSize

	// TimeFormat defines which pattern will be applied for
	// display time in the logs.
	TimeFormat = DefaultTimeFormat

	// Colors contain color array
	Colors = []string{
		MessageLog:  "\x1b[37m", // White
		Message2Log: "\x1b[92m", // Light green
		WarningLog:  "\x1b[93m", // Light Yellow
		DebugLog:    "\x1b[96m", // Light Cyan
		ErrorLog:    "\x1b[91m", // Light Red
	}

	// Prefixes of messages
	Prefixes = []string{
		MessageLog:  "msg",
		Message2Log: "msg",
		WarningLog:  "warning",
		DebugLog:    "debug",
		ErrorLog:    "error",
	}

	now      = time.Now
	adapters = make(map[string]AdapterPod)
	lock     = sync.RWMutex{}
)

func init() {
	if len(adapters) == 0 {
		AddAdapter("stdout", AdapterPod{
			Adapter: pln,
			Config:  nil,
		})
	}
}

// AddAdapter allows to add an adapter and parameters
func AddAdapter(name string, adapter AdapterPod) {
	lock.Lock()
	adapters[name] = adapter
	lock.Unlock()
}

// RemoveAapter remove the adapter from list
func RemoveAapter(name string) {
	lock.Lock()
	delete(adapters, name)
	lock.Unlock()
}

// SetAdapterConfig allows set new adapter parameters
func SetAdapterConfig(name string, config map[string]interface{}) {
	lock.Lock()
	a := adapters[name]
	a.Config = config
	adapters[name] = a
	lock.Unlock()
}

func runAdapters(m MsgType, o OutType, msg ...interface{}) {
	lock.RLock()
	defer lock.RUnlock()
	for _, a := range adapters {
		a.Adapter(m, o, a.Config, msg...)
	}
}

// HTTPError write lot to stdout and return json error on http.ResponseWriter with http error code.
func HTTPError(w http.ResponseWriter, code int) {
	msg := http.StatusText(code)
	Errorln(msg)
	m := make(map[string]string)
	m["status"] = "error"
	m["error"] = msg
	b, _ := json.MarshalIndent(m, "", "\t")
	http.Error(w, string(b), code)
}

// Fatal show message with line break at the end and exit to OS.
func Fatal(msg ...interface{}) {
	runAdapters(ErrorLog, LineOut, msg...)
	os.Exit(-1)
}

// Errorln message with line break at the end.
func Errorln(msg ...interface{}) {
	runAdapters(ErrorLog, LineOut, msg...)
}

// Errorf shows formatted error message on stdout without line break at the end.
func Errorf(msg ...interface{}) {
	runAdapters(ErrorLog, FormattedOut, msg...)
}

// Warningln shows warning message on stdout with line break at the end.
func Warningln(msg ...interface{}) {
	runAdapters(WarningLog, LineOut, msg...)
}

// Warningf shows formatted warning message on stdout without line break at the end.
func Warningf(msg ...interface{}) {
	runAdapters(WarningLog, FormattedOut, msg...)
}

// Println shows message on stdout with line break at the end.
func Println(msg ...interface{}) {
	runAdapters(MessageLog, LineOut, msg...)
}

// Printf shows formatted message on stdout without line break at the end.
func Printf(msg ...interface{}) {
	runAdapters(MessageLog, FormattedOut, msg...)
}

// Debugln shows debug message on stdout with line break at the end.
// If debug mode is not active no message is displayed
func Debugln(msg ...interface{}) {
	runAdapters(DebugLog, LineOut, msg...)
}

// Debugf shows debug message on stdout without line break at the end.
// If debug mode is not active no message is displayed
func Debugf(msg ...interface{}) {
	runAdapters(DebugLog, FormattedOut, msg...)
}

func pln(m MsgType, o OutType, config map[string]interface{}, msg ...interface{}) {
	if m == DebugLog && !DebugMode {
		return
	}

	var debugInfo, lineBreak, output string

	if DebugMode {
		_, fn, line, _ := runtime.Caller(5)
		fn = filepath.Base(fn)
		debugInfo = fmt.Sprintf("%s:%d ", fn, line)
	}

	if o == FormattedOut {
		output = fmt.Sprintf(msg[0].(string), msg[1:]...)
	} else {
		output = fmt.Sprint(msg...)
		lineBreak = "\n"
	}

	if EnableANSIColors {
		output = fmt.Sprintf("%s%s [%s] %s%s\033[0;00m",
			Colors[m],
			now().Format(TimeFormat),
			Prefixes[m],
			debugInfo,
			output)
	} else {
		output = fmt.Sprintf("%s [%s] %s%s",
			now().Format(TimeFormat),
			Prefixes[m],
			debugInfo,
			output)
	}

	if len(output) > MaxLineSize {
		output = output[:MaxLineSize] + "..."
	}
	output = output + lineBreak
	fmt.Print(output)
}
