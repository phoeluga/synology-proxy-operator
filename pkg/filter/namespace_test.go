package filter

import (
	"sync"
	"testing"
)

func TestNewNamespaceFilter_MatchAll(t *testing.T) {
	tests := []struct {
		name     string
		patterns string
	}{
		{"empty string", ""},
		{"asterisk", "*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := NewNamespaceFilter(tt.patterns)
			
			// Should match any namespace
			namespaces := []string{"default", "kube-system", "app-prod", "test-123"}
			for _, ns := range namespaces {
				if !filter.Matches(ns) {
					t.Errorf("Expected match-all filter to match %s", ns)
				}
			}
		})
	}
}

func TestNewNamespaceFilter_ExactMatch(t *testing.T) {
	filter := NewNamespaceFilter("default,kube-system")

	tests := []struct {
		namespace string
		want      bool
	}{
		{"default", true},
		{"kube-system", true},
		{"app-prod", false},
		{"default-test", false},
		{"kube-system-extra", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := filter.Matches(tt.namespace)
			if got != tt.want {
				t.Errorf("Matches(%s) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestNewNamespaceFilter_WildcardPrefix(t *testing.T) {
	filter := NewNamespaceFilter("app-*")

	tests := []struct {
		namespace string
		want      bool
	}{
		{"app-prod", true},
		{"app-dev", true},
		{"app-test", true},
		{"app-", true},
		{"app", false},
		{"myapp-prod", false},
		{"prod-app", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := filter.Matches(tt.namespace)
			if got != tt.want {
				t.Errorf("Matches(%s) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestNewNamespaceFilter_WildcardSuffix(t *testing.T) {
	filter := NewNamespaceFilter("*-prod")

	tests := []struct {
		namespace string
		want      bool
	}{
		{"app-prod", true},
		{"service-prod", true},
		{"test-prod", true},
		{"-prod", true},
		{"prod", false},
		{"app-prod-extra", false},
		{"production", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := filter.Matches(tt.namespace)
			if got != tt.want {
				t.Errorf("Matches(%s) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestNewNamespaceFilter_WildcardMiddle(t *testing.T) {
	filter := NewNamespaceFilter("app-*-prod")

	tests := []struct {
		namespace string
		want      bool
	}{
		{"app-test-prod", true},
		{"app-dev-prod", true},
		{"app-staging-prod", true},
		{"app--prod", true},
		{"app-prod", false},
		{"app-test-dev", false},
		{"myapp-test-prod", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := filter.Matches(tt.namespace)
			if got != tt.want {
				t.Errorf("Matches(%s) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestNewNamespaceFilter_MultiplePatterns(t *testing.T) {
	filter := NewNamespaceFilter("default,app-*,*-prod")

	tests := []struct {
		namespace string
		want      bool
	}{
		{"default", true},
		{"app-dev", true},
		{"app-test", true},
		{"service-prod", true},
		{"test-prod", true},
		{"kube-system", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := filter.Matches(tt.namespace)
			if got != tt.want {
				t.Errorf("Matches(%s) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestNewNamespaceFilter_WhitespaceHandling(t *testing.T) {
	filter := NewNamespaceFilter(" default , app-* , *-prod ")

	tests := []struct {
		namespace string
		want      bool
	}{
		{"default", true},
		{"app-dev", true},
		{"service-prod", true},
		{" default", false}, // Namespace names don't have leading spaces
		{"default ", false}, // Namespace names don't have trailing spaces
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := filter.Matches(tt.namespace)
			if got != tt.want {
				t.Errorf("Matches(%s) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}

func TestNewNamespaceFilter_Patterns(t *testing.T) {
	patterns := "default,app-*,*-prod"
	filter := NewNamespaceFilter(patterns)

	got := filter.Patterns()
	expected := []string{"default", "app-*", "*-prod"}

	if len(got) != len(expected) {
		t.Errorf("Patterns() length = %d, want %d", len(got), len(expected))
		return
	}

	for i, pattern := range expected {
		if got[i] != pattern {
			t.Errorf("Patterns()[%d] = %s, want %s", i, got[i], pattern)
		}
	}
}

func TestNewNamespaceFilter_RegexCaching(t *testing.T) {
	filter := NewNamespaceFilter("app-*").(*namespaceFilter)

	// First match should compile regex
	filter.Matches("app-prod")

	// Check that regex was cached
	if len(filter.compiled) != 1 {
		t.Errorf("Expected 1 compiled regex, got %d", len(filter.compiled))
	}

	// Second match should use cached regex
	filter.Matches("app-dev")

	// Should still have only 1 compiled regex
	if len(filter.compiled) != 1 {
		t.Errorf("Expected 1 compiled regex after second match, got %d", len(filter.compiled))
	}
}

func TestNewNamespaceFilter_ThreadSafety(t *testing.T) {
	filter := NewNamespaceFilter("app-*,*-prod")

	// Test concurrent access
	var wg sync.WaitGroup
	namespaces := []string{
		"app-prod", "app-dev", "service-prod", "test-prod",
		"app-test", "data-prod", "app-staging", "api-prod",
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			ns := namespaces[iteration%len(namespaces)]
			filter.Matches(ns)
		}(i)
	}

	wg.Wait()

	// If we get here without panic, thread safety test passed
}

func TestNewNamespaceFilter_EmptyPattern(t *testing.T) {
	filter := NewNamespaceFilter("")

	// Empty pattern should match all
	if !filter.Matches("any-namespace") {
		t.Error("Expected empty pattern to match all namespaces")
	}
}

func TestNewNamespaceFilter_ComplexWildcard(t *testing.T) {
	filter := NewNamespaceFilter("*app*prod*")

	tests := []struct {
		namespace string
		want      bool
	}{
		{"myapp-test-prod-1", true},
		{"app-prod", true},
		{"application-production", true},
		{"app", false},
		{"prod", false},
		{"test-dev", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			got := filter.Matches(tt.namespace)
			if got != tt.want {
				t.Errorf("Matches(%s) = %v, want %v", tt.namespace, got, tt.want)
			}
		})
	}
}
