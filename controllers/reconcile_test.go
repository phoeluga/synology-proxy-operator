package controllers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestShouldReconcile(t *testing.T) {
	tests := []struct {
		name    string
		ingress *networkingv1.Ingress
		want    bool
	}{
		{
			name: "enabled annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationEnabled: "true",
					},
				},
			},
			want: true,
		},
		{
			name: "disabled annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						AnnotationEnabled: "false",
					},
				},
			},
			want: false,
		},
		{
			name: "no annotation",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			want: false,
		},
		{
			name: "nil annotations",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldReconcile(tt.ingress)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractHostnames(t *testing.T) {
	tests := []struct {
		name    string
		ingress *networkingv1.Ingress
		want    []string
	}{
		{
			name: "single host",
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: "example.com"},
					},
				},
			},
			want: []string{"example.com"},
		},
		{
			name: "multiple hosts",
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: "example.com"},
						{Host: "test.com"},
					},
				},
			},
			want: []string{"example.com", "test.com"},
		},
		{
			name: "empty host",
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{Host: ""},
					},
				},
			},
			want: []string{},
		},
		{
			name: "no rules",
			ingress: &networkingv1.Ingress{
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{},
				},
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHostnames(tt.ingress)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		ingress     *networkingv1.Ingress
		key         string
		defaultVal  string
		want        string
	}{
		{
			name: "annotation exists",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"test-key": "test-value",
					},
				},
			},
			key:        "test-key",
			defaultVal: "default",
			want:       "test-value",
		},
		{
			name: "annotation missing - use default",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			key:        "missing-key",
			defaultVal: "default",
			want:       "default",
		},
		{
			name: "nil annotations - use default",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{},
			},
			key:        "test-key",
			defaultVal: "default",
			want:       "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAnnotation(tt.ingress, tt.key, tt.defaultVal)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildProxyRecordName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		ingressName string
		hostname  string
		want      string
	}{
		{
			name:        "standard case",
			namespace:   "default",
			ingressName: "my-ingress",
			hostname:    "example.com",
			want:        "default-my-ingress-example.com",
		},
		{
			name:        "with special characters",
			namespace:   "test-ns",
			ingressName: "test-ingress",
			hostname:    "sub.example.com",
			want:        "test-ns-test-ingress-sub.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildProxyRecordName(tt.namespace, tt.ingressName, tt.hostname)
			assert.Equal(t, tt.want, got)
		})
	}
}
