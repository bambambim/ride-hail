package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelDebug LogLevel = "DEBUG"
	LevelError LogLevel = "ERROR"
)

type LogFields map[string]interface{}

type Logger interface {
	WithFields(fields LogFields) Logger

	Info(action, message string)
	Debug(action, message string)
	Error(action string, err error)
}

// jsonLogger is the concrete implementation for logging in JSON format.
type jsonLogger struct {
	mu         sync.Mutex // Ensures concurrent writes are safe
	out        *os.File   // Output destination
	service    string     // The name of the service (e.g., "ride-service")
	hostname   string     // Hostname of the machine
	baseFields LogFields  // Fields to include in every log entry (e.g., ride_id)
}

// logEntry represents the structure of our JSON log.
// We use omitempty to keep the log entries clean.
type logEntry struct {
	Timestamp string   `json:"timestamp"`
	Level     LogLevel `json:"level"`
	Service   string   `json:"service"`
	Action    string   `json:"action"`
	Message   string   `json:"message"`
	Hostname  string   `json:"hostname"`
	RequestID string   `json:"request_id,omitempty"`
	RideID    string   `json:"ride_id,omitempty"`

	// Error details
	Error *errorEntry `json:"error,omitempty"`

	// Other dynamic fields
	Fields LogFields `json:"fields,omitempty"`
}

// errorEntry contains formatted error information.
type errorEntry struct {
	Msg   string `json:"msg"`
	Stack string `json:"stack"`
}

// NewLogger creates a new structured JSON logger for a specific service.
func NewLogger(serviceName string) Logger {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}

	return &jsonLogger{
		out:        os.Stdout,
		service:    serviceName,
		hostname:   host,
		baseFields: make(LogFields),
	}
}

// WithFields creates a new logger instance that inherits the base fields
// and adds the new fields.
func (l *jsonLogger) WithFields(fields LogFields) Logger {
	// Create a new map and copy base fields
	newFields := make(LogFields)
	for k, v := range l.baseFields {
		newFields[k] = v
	}

	// Add new fields, overwriting if keys conflict
	for k, v := range fields {
		newFields[k] = v
	}

	return &jsonLogger{
		out:        l.out,
		service:    l.service,
		hostname:   l.hostname,
		baseFields: newFields,
	}
}

// Info logs a message at the INFO level.
func (l *jsonLogger) Info(action, message string) {
	l.log(LevelInfo, action, message, nil)
}

// Debug logs a message at the DEBUG level.
func (l *jsonLogger) Debug(action, message string) {
	// In a real app, you might check a log-level config here
	l.log(LevelDebug, action, message, nil)
}

// Error logs an error, including a stack trace.
func (l *jsonLogger) Error(action string, err error) {
	// Capture stack trace
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Clean up the stack trace to be more readable
	stack = cleanStack(stack)

	errData := &errorEntry{
		Msg:   err.Error(),
		Stack: stack,
	}
	l.log(LevelError, action, err.Error(), errData)
}

// log is the internal method that constructs and writes the log entry.
func (l *jsonLogger) log(level LogLevel, action, message string, errData *errorEntry) {
	entry := &logEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano), // ISO 8601
		Level:     level,
		Service:   l.service,
		Action:    action,
		Message:   message,
		Hostname:  l.hostname,
		Error:     errData,
		Fields:    make(LogFields),
	}

	// Add base fields to the entry, handling specific known fields
	for k, v := range l.baseFields {
		switch k {
		case "ride_id":
			if rideID, ok := v.(string); ok {
				entry.RideID = rideID
			}
		case "request_id":
			if reqID, ok := v.(string); ok {
				entry.RequestID = reqID
			}
		default:
			// Put other fields in the generic 'fields' map
			entry.Fields[k] = v
		}
	}

	// If the fields map is empty, omit it from the JSON
	if len(entry.Fields) == 0 {
		entry.Fields = nil
	}

	// Marshal to JSON
	line, err := json.Marshal(entry)
	if err != nil {
		// Fallback to plain text if JSON marshaling fails
		fmt.Fprintf(os.Stderr, "Failed to marshal log: %v\n", err)
		fmt.Fprintf(l.out, "%s [%s] %s: %s (error: %v)\n", entry.Timestamp, entry.Level, entry.Action, entry.Message, entry.Error)
		return
	}

	// Lock for safe concurrent writes and print the log line
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintln(l.out, string(line))
}

// cleanStack simplifies the stack trace, removing Go internals.
func cleanStack(stack string) string {
	lines := strings.Split(stack, "\n")
	var cleaned []string

	// Add the first line (goroutine info)
	if len(lines) > 0 {
		cleaned = append(cleaned, lines[0])
	}

	for i := 1; i < len(lines); i += 2 {
		if i+1 >= len(lines) {
			break
		}

		// line 1: file path (e.g., /usr/local/go/src/runtime/panic.go:1038 +0x215)
		// line 2: function name (e.g., runtime.gopanic(0x10f13a0, 0x11a3d10))
		funcName := lines[i]
		filePath := lines[i+1]

		// Skip runtime and testing internals
		if strings.HasPrefix(funcName, "runtime.") ||
			strings.HasPrefix(funcName, "testing.") ||
			strings.Contains(funcName, "logger.Error") || // Don't include our own logger
			strings.Contains(filePath, "runtime/panic.go") {
			continue
		}

		// Trim file path
		filePath = strings.TrimSpace(filePath)

		cleaned = append(cleaned, funcName)
		cleaned = append(cleaned, "    "+filePath)
	}

	return strings.Join(cleaned, "\n")
}
