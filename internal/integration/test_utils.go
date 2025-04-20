// Package integration provides integration tests for the architect package
package integration

// No imports needed

// ContextKey defines a type for context keys to avoid string collisions in context values
type ContextKey string

// Common context keys used across integration tests
const (
	ModelNameKey ContextKey = "modelName"
)
