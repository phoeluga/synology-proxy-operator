package filter

import (
	"regexp"
	"strings"
	"sync"
)

// NamespaceFilter filters namespaces based on patterns
type NamespaceFilter interface {
	Matches(namespace string) bool
	Patterns() []string
}

type namespaceFilter struct {
	patterns []string
	compiled map[string]*regexp.Regexp
	matchAll bool
	mu       sync.RWMutex
}

// NewNamespaceFilter creates a new namespace filter
func NewNamespaceFilter(patterns string) NamespaceFilter {
	if patterns == "" || patterns == "*" {
		return &namespaceFilter{matchAll: true}
	}

	parts := strings.Split(patterns, ",")
	trimmed := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			trimmed = append(trimmed, t)
		}
	}

	return &namespaceFilter{
		patterns: trimmed,
		compiled: make(map[string]*regexp.Regexp),
	}
}

// Matches checks if a namespace matches any pattern
func (f *namespaceFilter) Matches(namespace string) bool {
	if f.matchAll {
		return true
	}

	for _, pattern := range f.patterns {
		if f.matchPattern(pattern, namespace) {
			return true
		}
	}

	return false
}

// matchPattern checks if a namespace matches a specific pattern
func (f *namespaceFilter) matchPattern(pattern, namespace string) bool {
	// Exact match (no wildcard)
	if !strings.Contains(pattern, "*") {
		return pattern == namespace
	}

	// Wildcard match - check cache first
	f.mu.RLock()
	regex, cached := f.compiled[pattern]
	f.mu.RUnlock()

	if cached {
		return regex.MatchString(namespace)
	}

	// Compile and cache
	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring write lock
	if regex, cached := f.compiled[pattern]; cached {
		return regex.MatchString(namespace)
	}

	// Convert wildcard to regex
	regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
	regex = regexp.MustCompile(regexPattern)
	f.compiled[pattern] = regex

	return regex.MatchString(namespace)
}

// Patterns returns the configured patterns
func (f *namespaceFilter) Patterns() []string {
	if f.matchAll {
		return []string{"*"}
	}
	return f.patterns
}
