package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

type Logger struct {
	output *os.File
}

type LogEntry struct {
	Timestamp string      `json:"timestamp"`
	Level     string      `json:"level"`
	Message   string      `json:"message"`
	File      string      `json:"file,omitempty"`
	Line      int         `json:"line,omitempty"`
	Fields    interface{} `json:"fields,omitempty"`
}

func NewLogger() *Logger {
	return &Logger{
		output: os.Stdout,
	}
}

func (l *Logger) log(level, msg string, fields ...interface{}) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "unknown"
		line = 0
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().UTC().Format(time.RFC3339),
		Level:     level,
		Message:   msg,
		File:      file,
		Line:      line,
	}

	if len(fields) > 0 && len(fields)%2 == 0 {
		fieldMap := make(map[string]interface{})
		for i := 0; i < len(fields); i += 2 {
			key, ok := fields[i].(string)
			if ok {
				fieldMap[key] = fields[i+1]
			}
		}
		entry.Fields = fieldMap
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling log entry: %v\n", err)
		return
	}

	fmt.Fprintln(l.output, string(jsonData))
}

func (l *Logger) Debug(msg string, fields ...interface{}) {
	l.log("DEBUG", msg, fields...)
}

func (l *Logger) Info(msg string, fields ...interface{}) {
	l.log("INFO", msg, fields...)
}

func (l *Logger) Warn(msg string, fields ...interface{}) {
	l.log("WARN", msg, fields...)
}

func (l *Logger) Error(msg string, fields ...interface{}) {
	l.log("ERROR", msg, fields...)
}

func (l *Logger) Fatal(msg string, fields ...interface{}) {
	l.log("FATAL", msg, fields...)
	os.Exit(1)
}

func (l *Logger) WithCorrelationID(correlationID string) *Logger {
	return l
}

func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l
}
