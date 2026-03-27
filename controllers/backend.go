package controllers

import (
	"fmt"
	"strings"

	"github.com/phoeluga/synology-proxy-operator/pkg/logging"
	networkingv1 "k8s.io/api/networking/v1"
)

// BackendTarget represents a discovered backend service
type BackendTarget struct {
	ServiceName      string
	ServiceNamespace string
	ServicePort      int
	Protocol         string // http or https
	FQDN             string // Fully qualified domain name
}

// BackendDiscovery discovers backend service from Ingress
type BackendDiscovery struct {
	logger logging.Logger
}

// NewBackendDiscovery creates a new BackendDiscovery
func NewBackendDiscovery(logger logging.Logger) *BackendDiscovery {
	return &BackendDiscovery{
		logger: logger,
	}
}

// Discover extracts backend information from Ingress
func (d *BackendDiscovery) Discover(ingress *networkingv1.Ingress) (*BackendTarget, error) {
	if len(ingress.Spec.Rules) == 0 {
		return nil, fmt.Errorf("no rules in Ingress spec")
	}

	rule := ingress.Spec.Rules[0]
	if rule.HTTP == nil || len(rule.HTTP.Paths) == 0 {
		return nil, fmt.Errorf("no HTTP paths in Ingress spec")
	}

	path := rule.HTTP.Paths[0]
	backend := path.Backend

	if backend.Service == nil {
		return nil, fmt.Errorf("no service backend in Ingress spec")
	}

	serviceName := backend.Service.Name
	if serviceName == "" {
		return nil, fmt.Errorf("service name is empty")
	}

	// Get service port
	var servicePort int
	if backend.Service.Port.Number != 0 {
		servicePort = int(backend.Service.Port.Number)
	} else if backend.Service.Port.Name != "" {
		// Named port - we can't resolve it without querying the Service
		// For now, default to 80 for HTTP and 443 for HTTPS
		protocol := d.getProtocol(ingress)
		if protocol == "https" {
			servicePort = 443
		} else {
			servicePort = 80
		}
		d.logger.Warn("Named service port not supported, using default",
			"port_name", backend.Service.Port.Name,
			"default_port", servicePort)
	} else {
		return nil, fmt.Errorf("service port not specified")
	}

	// Construct FQDN
	fqdn := fmt.Sprintf("%s.%s.svc.cluster.local", serviceName, ingress.Namespace)

	// Determine protocol (default HTTP)
	protocol := d.getProtocol(ingress)

	target := &BackendTarget{
		ServiceName:      serviceName,
		ServiceNamespace: ingress.Namespace,
		ServicePort:      servicePort,
		Protocol:         protocol,
		FQDN:             fqdn,
	}

	d.logger.Debug("Backend discovered",
		"service", serviceName,
		"namespace", ingress.Namespace,
		"port", servicePort,
		"protocol", protocol,
		"fqdn", fqdn)

	return target, nil
}

// getProtocol determines the backend protocol from annotations
func (d *BackendDiscovery) getProtocol(ingress *networkingv1.Ingress) string {
	if ingress.Annotations == nil {
		return "http"
	}

	if proto, ok := ingress.Annotations[BackendProtocolAnnotation]; ok {
		protocol := strings.ToLower(strings.TrimSpace(proto))
		if protocol == "http" || protocol == "https" {
			return protocol
		}
		d.logger.Warn("Invalid backend protocol annotation, defaulting to http",
			"annotation_value", proto)
	}

	return "http"
}

// ValidateBackend validates the backend configuration
func (d *BackendDiscovery) ValidateBackend(target *BackendTarget) error {
	if target == nil {
		return fmt.Errorf("backend target is nil")
	}

	if target.ServiceName == "" {
		return fmt.Errorf("service name is empty")
	}

	if target.ServiceNamespace == "" {
		return fmt.Errorf("service namespace is empty")
	}

	if target.ServicePort <= 0 || target.ServicePort > 65535 {
		return fmt.Errorf("invalid service port: %d", target.ServicePort)
	}

	if target.Protocol != "http" && target.Protocol != "https" {
		return fmt.Errorf("invalid protocol: %s", target.Protocol)
	}

	if target.FQDN == "" {
		return fmt.Errorf("FQDN is empty")
	}

	return nil
}
