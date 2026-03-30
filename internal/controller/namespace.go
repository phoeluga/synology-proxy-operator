package controller

import "path"

// namespaceMatches reports whether namespace matches the given pattern.
// Supports glob syntax: "*" matches any sequence of characters, "?" matches
// any single character, "[abc]" matches a character class.
// Examples: "app-*" matches "app-foo", "app-bar"; "prod-?" matches "prod-1".
// An empty pattern never matches — callers interpret that as "annotation-only mode".
func namespaceMatches(namespace, pattern string) bool {
	if pattern == "" {
		return false
	}
	matched, err := path.Match(pattern, namespace)
	return err == nil && matched
}
