package watcher

import (
	"context"
	"fmt"

	"github.com/phoeluga/synology-proxy-operator/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CredentialChangeCallback is called when credentials change
type CredentialChangeCallback func(username, password string) error

// SecretWatcher watches a Kubernetes Secret for changes
type SecretWatcher interface {
	Start(ctx context.Context) error
	Stop()
}

type secretWatcher struct {
	client    client.Client
	secretRef types.NamespacedName
	onChange  CredentialChangeCallback
	stopCh    chan struct{}
	logger    logging.Logger
}

// NewSecretWatcher creates a new secret watcher
func NewSecretWatcher(
	client client.Client,
	secretName, secretNamespace string,
	onChange CredentialChangeCallback,
	logger logging.Logger,
) SecretWatcher {
	return &secretWatcher{
		client: client,
		secretRef: types.NamespacedName{
			Name:      secretName,
			Namespace: secretNamespace,
		},
		onChange: onChange,
		stopCh:   make(chan struct{}),
		logger:   logger,
	}
}

// Start starts watching the Secret
func (w *secretWatcher) Start(ctx context.Context) error {
	w.logger.Info("Starting Secret watcher",
		"secret", w.secretRef.Name,
		"namespace", w.secretRef.Namespace,
	)

	// TODO: Implement Secret watch using controller-runtime
	// This is a placeholder implementation
	// In a full implementation, you would:
	// 1. Set up a watch on the Secret resource
	// 2. Handle Secret update events
	// 3. Call handleSecretUpdate when the Secret changes

	// For now, just log that the watcher is started
	w.logger.Info("Secret watcher started (watch implementation pending)")

	return nil
}

// Stop stops the Secret watcher
func (w *secretWatcher) Stop() {
	w.logger.Info("Stopping Secret watcher")
	close(w.stopCh)
}

// handleSecretUpdate handles Secret update events
func (w *secretWatcher) handleSecretUpdate(secret *corev1.Secret) {
	username := string(secret.Data["username"])
	password := string(secret.Data["password"])

	if username == "" || password == "" {
		w.logger.Error("Secret missing username or password", fmt.Errorf("invalid credentials"))
		return
	}

	w.logger.Info("Credentials changed, reloading")

	if err := w.onChange(username, password); err != nil {
		w.logger.Error("Failed to reload credentials", err)
	} else {
		w.logger.Info("Credentials reloaded successfully")
	}
}
