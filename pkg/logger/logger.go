package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/SIMPLYBOYS/trading_ace/internal/errors"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var logLevelNames = map[LogLevel]string{
	DEBUG: "DEBUG",
	INFO:  "INFO",
	WARN:  "WARN",
	ERROR: "ERROR",
	FATAL: "FATAL",
}

type Logger struct {
	level      LogLevel
	output     io.Writer
	logger     *log.Logger
	fileLogger *log.Logger
	mu         sync.Mutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func init() {
	once.Do(func() {
		defaultLogger = NewLogger(INFO, os.Stdout)
	})
}

// NewLogger creates a new Logger instance
func NewLogger(level LogLevel, output io.Writer) *Logger {
	return &Logger{
		level:  level,
		output: output,
		logger: log.New(output, "", log.Ldate|log.Ltime),
	}
}

// SetLevel sets the logging level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// EnableFileLogging enables logging to a file
func (l *Logger) EnableFileLogging(directory string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(directory, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile := filepath.Join(directory, fmt.Sprintf("app_%s.log", time.Now().Format("2006-01-02")))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.fileLogger = log.New(file, "", log.Ldate|log.Ltime)
	return nil
}

func (l *Logger) log(level LogLevel, format string, v ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	_, file, line, _ := runtime.Caller(2)
	msg := fmt.Sprintf(format, v...)
	logMsg := fmt.Sprintf("[%s] [%s:%d] %s", logLevelNames[level], filepath.Base(file), line, msg)

	l.logger.Println(logMsg)
	if l.fileLogger != nil {
		l.fileLogger.Println(logMsg)
	}

	if level == FATAL {
		os.Exit(1)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(DEBUG, format, v...)
}

// Info logs an info message
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(INFO, format, v...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(WARN, format, v...)
}

// Error logs an error message
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(ERROR, format, v...)
}

// Fatal logs a fatal message and exits the program
func (l *Logger) Fatal(format string, v ...interface{}) {
	l.log(FATAL, format, v...)
}

// Errorf logs an error message and returns an error
func (l *Logger) Errorf(err error, format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)
	wrappedErr := fmt.Errorf("%s: %w", msg, err)
	l.Error(wrappedErr.Error())
	return wrappedErr
}

// Global functions that use the default logger

// SetLevel sets the logging level for the default logger
func SetLevel(level LogLevel) {
	defaultLogger.SetLevel(level)
}

// EnableFileLogging enables file logging for the default logger
func EnableFileLogging(directory string) error {
	return defaultLogger.EnableFileLogging(directory)
}

// Debug logs a debug message using the default logger
func Debug(format string, v ...interface{}) {
	defaultLogger.Debug(format, v...)
}

// Info logs an info message using the default logger
func Info(format string, v ...interface{}) {
	defaultLogger.Info(format, v...)
}

// Warn logs a warning message using the default logger
func Warn(format string, v ...interface{}) {
	defaultLogger.Warn(format, v...)
}

// Error logs an error message using the default logger
func Error(format string, v ...interface{}) {
	defaultLogger.Error(format, v...)
}

// Fatal logs a fatal message and exits the program using the default logger
func Fatal(format string, v ...interface{}) {
	defaultLogger.Fatal(format, v...)
}

// Errorf logs an error message and returns an error using the default logger
func Errorf(err error, format string, v ...interface{}) error {
	return defaultLogger.Errorf(err, format, v...)
}

func logError(err error) {
	switch e := err.(type) {
	case *errors.DatabaseError:
		Error("Database error during %s: %v", e.Operation, e.Err)
	case *errors.EthereumError:
		Error("Ethereum error during %s: %v", e.Operation, e.Err)
	case *errors.APIError:
		Error("API error (status %d): %s - %v", e.StatusCode, e.Message, e.Err)
	default:
		Error("Unexpected error: %v", err)
	}
}
