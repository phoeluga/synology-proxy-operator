package controller

import (
	"testing"
)

// ── namespaceMatches ─────────────────────────────────────────────────────────

func TestNamespaceMatches(t *testing.T) {
	tests := []struct {
		namespace string
		pattern   string
		want      bool
	}{
		// Empty pattern = annotation-only mode, never matches
		{"app-headlamp", "", false},
		{"anything", "", false},

		// Exact match
		{"production", "production", true},
		{"staging", "production", false},

		// Glob with *
		{"app-headlamp", "app-*", true},
		{"app-grafana", "app-*", true},
		{"monitoring", "app-*", false},
		{"app-", "app-*", true},

		// Glob with ?
		{"prod-1", "prod-?", true},
		{"prod-2", "prod-?", true},
		{"prod-12", "prod-?", false},

		// Wildcard only
		{"anything", "*", true},
		{"", "*", true},
	}

	for _, tt := range tests {
		got := namespaceMatches(tt.namespace, tt.pattern)
		if got != tt.want {
			t.Errorf("namespaceMatches(%q, %q) = %v, want %v", tt.namespace, tt.pattern, got, tt.want)
		}
	}
}

// ── ruleNameForObject ────────────────────────────────────────────────────────

func TestRuleNameForObject(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		// Double-dash separator prevents collision with names containing single dashes
		{"headlamp", "app-headlamp", "app-headlamp--headlamp"},
		{"nginx", "monitoring", "monitoring--nginx"},
		{"my-app", "my-namespace", "my-namespace--my-app"},
		// Same name, different namespaces → different rule names
		{"api", "frontend", "frontend--api"},
		{"api", "backend", "backend--api"},
	}

	for _, tt := range tests {
		got := ruleNameForObject(tt.name, tt.namespace)
		if got != tt.want {
			t.Errorf("ruleNameForObject(%q, %q) = %q, want %q", tt.name, tt.namespace, got, tt.want)
		}
	}
}

// ── isEnabled ────────────────────────────────────────────────────────────────

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{"nil annotations", nil, false},
		{"empty annotations", map[string]string{}, false},
		{"enabled true", map[string]string{AnnotationEnabled: "true"}, true},
		{"enabled TRUE (uppercase)", map[string]string{AnnotationEnabled: "TRUE"}, true},
		{"enabled True (mixed)", map[string]string{AnnotationEnabled: "True"}, true},
		{"enabled false", map[string]string{AnnotationEnabled: "false"}, false},
		{"enabled empty", map[string]string{AnnotationEnabled: ""}, false},
		{"other annotation only", map[string]string{AnnotationSourceHost: "foo.example.com"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEnabled(tt.annotations)
			if got != tt.want {
				t.Errorf("isEnabled(%v) = %v, want %v", tt.annotations, got, tt.want)
			}
		})
	}
}

// ── isResourceEnabled ────────────────────────────────────────────────────────

func TestIsResourceEnabled(t *testing.T) {
	r := &ServiceIngressReconciler{WatchNamespace: "app-*"}

	tests := []struct {
		name          string
		namespace     string
		annotations   map[string]string
		nsAnnotations map[string]string
		want          bool
	}{
		{
			name:        "annotation enabled, namespace not matching",
			namespace:   "monitoring",
			annotations: map[string]string{AnnotationEnabled: "true"},
			want:        true,
		},
		{
			name:        "no annotation, namespace matches glob",
			namespace:   "app-headlamp",
			annotations: map[string]string{},
			want:        true,
		},
		{
			name:        "no annotation, namespace does not match",
			namespace:   "monitoring",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name:        "annotation disabled, namespace matches glob — explicit opt-out always wins",
			namespace:   "app-foo",
			annotations: map[string]string{AnnotationEnabled: "false"},
			want:        false,
		},
		{
			name:          "no annotation, namespace matches glob but auto-discovery disabled on namespace",
			namespace:     "app-foo",
			annotations:   map[string]string{},
			nsAnnotations: map[string]string{AnnotationAutoDiscovery: "false"},
			want:          false,
		},
		{
			name:          "explicit opt-in overrides namespace auto-discovery=false",
			namespace:     "app-foo",
			annotations:   map[string]string{AnnotationEnabled: "true"},
			nsAnnotations: map[string]string{AnnotationAutoDiscovery: "false"},
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.isResourceEnabled(tt.namespace, tt.annotations, tt.nsAnnotations)
			if got != tt.want {
				t.Errorf("isResourceEnabled(%q, %v, %v) = %v, want %v", tt.namespace, tt.annotations, tt.nsAnnotations, got, tt.want)
			}
		})
	}
}
