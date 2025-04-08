package logger

import (
	"log"
)

var isDebug = false

// SetDebug enables or disables debug logging
func SetDebug(debug bool) {
	isDebug = debug
}

// Debug logs a debug-level message
func Debug(format string, v ...interface{}) {
	if isDebug {
		log.Printf("DEBUG: "+format, v...)
	}
}

// Info logs an info-level message
func Info(format string, v ...interface{}) {
	log.Printf("INFO: "+format, v...)
}

// Warn logs a warning-level message
func Warn(format string, v ...interface{}) {
	log.Printf("WARN: "+format, v...)
}

// Error logs an error-level message
func Error(format string, v ...interface{}) {
	log.Printf("ERROR: "+format, v...)
}
