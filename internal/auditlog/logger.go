// Package auditlog provides structured logging for audit purposes
package auditlog

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/phrazzld/thinktank/internal/llm"
	"github.com/phrazzld/thinktank/internal/logutil"
)

// AuditLogger defines the interface for logging audit events.
// Implementations of this interface will handle persisting audit
// log entries in various formats (e.g., JSON Lines file, no-op).
type AuditLogger interface {
	// Log records a single audit entry.
	// The entry contains information about operations, status, and relevant metadata.
	// Returns an error if the logging operation fails.
	// NOTE: Prefer using the LogOp method instead of this method directly.
	Log(entry AuditEntry) error

	// LogOp is a helper method for logging operations with minimal parameters.
	// It creates an AuditEntry with the given operation, status, and optional data,
	// sets a timestamp, and logs it. The method returns any error from logging.
	// This is the recommended way to log audit events to ensure consistency.
	LogOp(operation, status string, inputs map[string]interface{}, outputs map[string]interface{}, err error) error

	// Close releases any resources used by the logger (e.g., open file handles).
	// Should be called when the logger is no longer needed.
	// Returns an error if the closing operation fails.
	Close() error
}

// FileAuditLogger implements AuditLogger by writing JSON Lines to a file.
type FileAuditLogger struct {
	file   *os.File
	mu     sync.Mutex
	logger logutil.LoggerInterface // For logging errors within the audit logger itself
}

// NewFileAuditLogger creates a new FileAuditLogger that writes to the specified file path.
// If the file doesn't exist, it will be created. If it does exist, logs will be appended.
// The provided internal logger is used to log any errors that occur during audit logging operations.
func NewFileAuditLogger(filePath string, internalLogger logutil.LoggerInterface) (*FileAuditLogger, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		internalLogger.Error("Failed to open audit log file '%s': %v", filePath, err)
		return nil, fmt.Errorf("failed to open audit log file %s: %w", filePath, err)
	}
	internalLogger.Info("Audit logging enabled to file: %s", filePath)
	return &FileAuditLogger{
		file:   file,
		logger: internalLogger,
	}, nil
}

// Log records a single audit entry by marshaling it to JSON and writing it to the log file.
// It sets the entry timestamp if not already set and ensures thread safety with a mutex lock.
func (l *FileAuditLogger) Log(entry AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Ensure timestamp is set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	// Marshal entry to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		l.logger.Error("Failed to marshal audit entry to JSON: %v, Entry: %+v", err, entry)
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Write JSON line to file
	if _, err := l.file.Write(append(jsonData, '\n')); err != nil {
		l.logger.Error("Failed to write audit entry to file '%s': %v", l.file.Name(), err)
		return fmt.Errorf("failed to write audit entry: %w", err)
	}
	return nil
}

// LogOp implements the AuditLogger interface's LogOp method.
// It creates an AuditEntry with the provided parameters and logs it.
func (l *FileAuditLogger) LogOp(operation, status string, inputs map[string]interface{}, outputs map[string]interface{}, err error) error {
	// Create a new entry with current timestamp
	entry := AuditEntry{
		Timestamp: time.Now().UTC(),
		Operation: operation,
		Status:    status,
		Inputs:    inputs,
		Outputs:   outputs,
	}

	// Add message based on status and operation
	var message string
	switch status {
	case "Success":
		message = fmt.Sprintf("%s completed successfully", operation)
	case "InProgress":
		message = fmt.Sprintf("%s started", operation)
	case "Failure":
		message = fmt.Sprintf("%s failed", operation)
	default:
		message = fmt.Sprintf("%s - %s", operation, status)
	}
	entry.Message = message

	// Add error information if provided
	if err != nil {
		errorType := "GeneralError"

		// Extract error category for categorized errors
		if catErr, ok := llm.IsCategorizedError(err); ok {
			category := catErr.Category()
			errorType = fmt.Sprintf("Error:%s", category.String())
		}

		entry.Error = &ErrorInfo{
			Message: err.Error(),
			Type:    errorType,
		}
	}

	// Log the entry
	return l.Log(entry)
}

// Close properly closes the log file.
// It ensures thread safety with a mutex lock and prevents double-closing.
func (l *FileAuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		l.logger.Info("Closing audit log file: %s", l.file.Name())
		err := l.file.Close()
		l.file = nil // Prevent double close
		if err != nil {
			l.logger.Error("Error closing audit log file: %v", err)
			return err
		}
	}
	return nil
}

// NoOpAuditLogger implements AuditLogger with no-op methods.
// This implementation is used when audit logging is disabled.
type NoOpAuditLogger struct{}

// NewNoOpAuditLogger creates a new NoOpAuditLogger instance.
func NewNoOpAuditLogger() *NoOpAuditLogger {
	return &NoOpAuditLogger{}
}

// Log implements the AuditLogger interface but performs no action.
// It always returns nil (no error).
func (l *NoOpAuditLogger) Log(entry AuditEntry) error {
	return nil // Do nothing
}

// LogOp implements the AuditLogger interface's LogOp method but performs no action.
// It always returns nil (no error).
func (l *NoOpAuditLogger) LogOp(operation, status string, inputs map[string]interface{}, outputs map[string]interface{}, err error) error {
	return nil // Do nothing
}

// Close implements the AuditLogger interface but performs no action.
// It always returns nil (no error).
func (l *NoOpAuditLogger) Close() error {
	return nil // Do nothing
}

// Compile-time checks to ensure implementations satisfy the AuditLogger interface.
var _ AuditLogger = (*FileAuditLogger)(nil)
var _ AuditLogger = (*NoOpAuditLogger)(nil)
