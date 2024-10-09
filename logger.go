package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
)

var (
	infoLogger  *log.Logger
	errorLogger *log.Logger
)

func init() {
	infoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime)
	errorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime)
}

func LogInfo(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	_, file, line, _ := runtime.Caller(1)
	infoLogger.Printf("[%s:%d] %s", file, line, msg)
}

func LogError(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	_, file, line, _ := runtime.Caller(1)
	errorLogger.Printf("[%s:%d] %s", file, line, msg)
}

func LogErrorf(err error, format string, v ...interface{}) error {
	msg := fmt.Sprintf(format, v...)
	wrappedErr := fmt.Errorf("%s: %w", msg, err)
	LogError(wrappedErr.Error())
	return wrappedErr
}

func LogFatal(format string, v ...interface{}) {
	LogError(format, v...)
	os.Exit(1)
}
