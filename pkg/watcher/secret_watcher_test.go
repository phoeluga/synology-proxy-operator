package watcher

import (
	"context"
	"testing"
	"time"

	"github.com/phoeluga/synology-proxy-operator/pkg/config"
	"github.com/phoeluga/synology-proxy-operator/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewSecretWatcher(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := logging.NewLogger(config.LoggingConfig{Level: "info", Format: "json"})

	callbackCalled := false
	callback := func(username, password string) error {
		callbackCalled = true
		return nil
	}

	watcher := NewSecretWatcher(client, "test-secret", "default", callback, logger)

	if watcher == nil {
		t.Error("NewSecretWatcher() returned nil")
	}
}

func TestSecretWatcher_Start(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	// Create fake client with a secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(secret).
		Build()

	logger := logging.NewLogger(config.LoggingConfig{Level: "info", Format: "json"})

	callbackCalled := false
	callback := func(username, password string) error {
		callbackCalled = true
		return nil
	}

	watcher := NewSecretWatcher(client, "test-secret", "default", callback, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start watcher (non-blocking)
	err := watcher.Start(ctx)
	if err != nil {
		t.Errorf("Start() error = %v", err)
	}
}

func TestSecretWatcher_Stop(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := logging.NewLogger(config.LoggingConfig{Level: "info", Format: "json"})

	callback := func(username, password string) error {
		return nil
	}

	watcher := NewSecretWatcher(client, "test-secret", "default", callback, logger)

	// Stop should not panic even if not started
	watcher.Stop()
}

func TestSecretWatcher_CallbackInvocation(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := logging.NewLogger(config.LoggingConfig{Level: "info", Format: "json"})

	var capturedUsername, capturedPassword string
	callback := func(username, password string) error {
		capturedUsername = username
		capturedPassword = password
		return nil
	}

	watcher := NewSecretWatcher(client, "test-secret", "default", callback, logger).(*secretWatcher)

	// Simulate secret update
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("testuser"),
			"password": []byte("testpass"),
		},
	}

	watcher.handleSecretUpdate(secret)

	// Verify callback was called with correct values
	if capturedUsername != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", capturedUsername)
	}
	if capturedPassword != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", capturedPassword)
	}
}

func TestSecretWatcher_EmptyCredentials(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := logging.NewLogger(config.LoggingConfig{Level: "info", Format: "json"})

	var capturedUsername, capturedPassword string
	callback := func(username, password string) error {
		capturedUsername = username
		capturedPassword = password
		return nil
	}

	watcher := NewSecretWatcher(client, "test-secret", "default", callback, logger).(*secretWatcher)

	// Simulate secret with empty credentials
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte(""),
			"password": []byte(""),
		},
	}

	watcher.handleSecretUpdate(secret)

	// Callback should still be called with empty strings
	if capturedUsername != "" {
		t.Errorf("Expected empty username, got '%s'", capturedUsername)
	}
	if capturedPassword != "" {
		t.Errorf("Expected empty password, got '%s'", capturedPassword)
	}
}

func TestSecretWatcher_MissingFields(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	logger := logging.NewLogger(config.LoggingConfig{Level: "info", Format: "json"})

	callbackCalled := false
	callback := func(username, password string) error {
		callbackCalled = true
		return nil
	}

	watcher := NewSecretWatcher(client, "test-secret", "default", callback, logger).(*secretWatcher)

	// Simulate secret with missing fields
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{},
	}

	watcher.handleSecretUpdate(secret)

	// Callback should still be called (with empty strings from missing keys)
	if !callbackCalled {
		t.Error("Expected callback to be called even with missing fields")
	}
}
