package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

var (
	logger       *log.Logger
	output       io.Writer
	level        Level
	mu           sync.Mutex
	logFile      *os.File
	callDepth    = 3
	enableColors bool
)

func init() {
	Setup(os.Stdout, DEBUG)
}

func Setup(w io.Writer, lvl Level) {
	output = w
	level = lvl
	logger = log.New(w, "", 0)
	enableColors = isTerminal(w)
}

func SetLevel(lvl Level) {
	mu.Lock()
	defer mu.Unlock()
	level = lvl
}

func SetCallDepth(d int) {
	mu.Lock()
	defer mu.Unlock()
	callDepth = d
}

func EnableColors(enable bool) {
	mu.Lock()
	defer mu.Unlock()
	enableColors = enable
}

func isTerminal(w io.Writer) bool {
	_, ok := w.(*os.File)
	if ok {
		return runtime.GOOS == "darwin" || runtime.GOOS == "linux"
	}
	return false
}

func colorForLevel(lvl Level) string {
	switch lvl {
	case DEBUG:
		return "\033[36m" // Cyan
	case INFO:
		return "\033[32m" // Green
	case WARN:
		return "\033[33m" // Yellow
	case ERROR:
		return "\033[31m" // Red
	default:
		return "\033[0m"
	}
}

func levelString(lvl Level) string {
	switch lvl {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

func formatMessage(format string, args ...interface{}) string {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	return msg
}

func logWithContext(lvl Level, format string, args ...interface{}) {
	if lvl < level {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	msg := formatMessage(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	_, file, line, ok := runtime.Caller(callDepth)
	var location string
	if ok {
		relPath, err := filepath.Rel(os.Getenv("PWD"), file)
		if err != nil {
			location = fmt.Sprintf("%s:%d", file, line)
		} else {
			location = fmt.Sprintf("%s:%d", relPath, line)
		}
	}

	if enableColors {
		color := colorForLevel(lvl)
		reset := "\033[0m"
		logger.Printf("%s[%s]%s %s %s → %s%s", color, timestamp, reset, levelString(lvl), location, msg, reset)
	} else {
		logger.Printf("[%s] %s %s → %s", timestamp, levelString(lvl), location, msg)
	}
}

func Debug(format string, args ...interface{}) {
	logWithContext(DEBUG, format, args...)
}

func Debugf(format string, args ...interface{}) {
	logWithContext(DEBUG, format, args...)
}

func Info(format string, args ...interface{}) {
	logWithContext(INFO, format, args...)
}

func Infof(format string, args ...interface{}) {
	logWithContext(INFO, format, args...)
}

func Warn(format string, args ...interface{}) {
	logWithContext(WARN, format, args...)
}

func Warnf(format string, args ...interface{}) {
	logWithContext(WARN, format, args...)
}

func Error(format string, args ...interface{}) {
	logWithContext(ERROR, format, args...)
}

func Errorf(format string, args ...interface{}) {
	logWithContext(ERROR, format, args...)
}

func Fatal(format string, args ...interface{}) {
	logWithContext(ERROR, format, args...)
	os.Exit(1)
}

func Fatalf(format string, args ...interface{}) {
	logWithContext(ERROR, format, args...)
	os.Exit(1)
}

func WithFields(fields map[string]interface{}) Logger {
	return &fieldsLogger{fields: fields}
}

type Logger interface {
	Debug(msg string)
	Info(msg string)
	Warn(msg string)
	Error(msg string)
}

type fieldsLogger struct {
	fields map[string]interface{}
}

func (f *fieldsLogger) formatMsg(msg string) string {
	var parts []string
	for k, v := range f.fields {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	if len(parts) > 0 {
		return fmt.Sprintf("%s {%s}", msg, strings.Join(parts, ", "))
	}
	return msg
}

func (f *fieldsLogger) Debug(msg string) {
	Debug(f.formatMsg(msg))
}

func (f *fieldsLogger) Info(msg string) {
	Info(f.formatMsg(msg))
}

func (f *fieldsLogger) Warn(msg string) {
	Warn(f.formatMsg(msg))
}

func (f *fieldsLogger) Error(msg string) {
	Error(f.formatMsg(msg))
}

func File(path string, lvl Level) error {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	logFile = f
	output = io.MultiWriter(os.Stdout, f)
	logger.SetOutput(output)
	level = lvl

	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}
