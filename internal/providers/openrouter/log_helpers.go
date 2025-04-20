// Package openrouter provides the implementation of the OpenRouter LLM provider
package openrouter

import (
	"net/url"
)

// SanitizeURL removes sensitive information from URLs before they are logged
// It removes any credentials embedded in the URL while preserving the rest
// of the URL structure for diagnostic purposes
func SanitizeURL(urlStr string) string {
	// Parse the URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// If we can't parse the URL, return a placeholder to avoid logging the raw string
		// which might contain sensitive information
		return "[unparsable-url]"
	}

	// Remove any userinfo (username:password) component from the URL
	if parsedURL.User != nil {
		parsedURL.User = nil
	}

	// Return the sanitized URL as a string
	return parsedURL.String()
}

// GetBaseURLLogInfo returns a sanitized logging representation of a base URL
// Similar to registry.getBaseURLLogSuffix but with sanitization
func GetBaseURLLogInfo(baseURL string) string {
	if baseURL == "" {
		return "default base URL"
	}
	return "custom base URL: " + SanitizeURL(baseURL)
}
