package controller

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// ── extractFromIngress ───────────────────────────────────────────────────────

func TestExtractFromIngress(t *testing.T) {
	tests := []struct {
		name     string
		tls      []networkingv1.IngressTLS
		lbStatus []networkingv1.IngressLoadBalancerIngress
		wantHost string
		wantPort int
	}{
		{
			name:     "no TLS, IP LoadBalancer -> port 80",
			lbStatus: []networkingv1.IngressLoadBalancerIngress{{IP: "10.0.0.1"}},
			wantHost: "10.0.0.1",
			wantPort: 80,
		},
		{
			name:     "TLS configured, IP LoadBalancer -> port 443",
			tls:      []networkingv1.IngressTLS{{Hosts: []string{"example.com"}}},
			lbStatus: []networkingv1.IngressLoadBalancerIngress{{IP: "10.0.0.1"}},
			wantHost: "10.0.0.1",
			wantPort: 443,
		},
		{
			name:     "TLS configured, Hostname LoadBalancer -> port 443",
			tls:      []networkingv1.IngressTLS{{Hosts: []string{"example.com"}}},
			lbStatus: []networkingv1.IngressLoadBalancerIngress{{Hostname: "lb.example.com"}},
			wantHost: "lb.example.com",
			wantPort: 443,
		},
		{
			name:     "no LoadBalancer status -> empty host, port 0",
			wantHost: "",
			wantPort: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ing := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec:       networkingv1.IngressSpec{TLS: tt.tls},
				Status: networkingv1.IngressStatus{
					LoadBalancer: networkingv1.IngressLoadBalancerStatus{Ingress: tt.lbStatus},
				},
			}
			gotHost, gotPort := extractFromIngress(ing)
			if gotHost != tt.wantHost || gotPort != tt.wantPort {
				t.Errorf("extractFromIngress() = (%q, %d), want (%q, %d)", gotHost, gotPort, tt.wantHost, tt.wantPort)
			}
		})
	}
}
