package controller

// Annotation keys recognised on Service, Ingress, and ArgoCD Application objects.
const (
	// AnnotationEnabled opts a resource into proxy management.
	// Value: "true" | "false"
	AnnotationEnabled = "synology.proxy/enabled"

	// AnnotationSourceHost overrides the public FQDN (frontend hostname).
	// When absent, the operator constructs it as <name>.<defaultDomain>.
	AnnotationSourceHost = "synology.proxy/source-host"

	// AnnotationACLProfile overrides the ACL profile name for this resource.
	AnnotationACLProfile = "synology.proxy/acl-profile"

	// AnnotationDestProtocol overrides the backend protocol ("http" or "https").
	AnnotationDestProtocol = "synology.proxy/destination-protocol"

	// AnnotationAssignCert controls certificate assignment ("true"/"false").
	AnnotationAssignCert = "synology.proxy/assign-certificate"

	// AnnotationServiceRef selects a specific Service for backend discovery.
	// Format: "<namespace>/<name>" or just "<name>" (namespace inferred).
	AnnotationServiceRef = "synology.proxy/service-ref"

	// AnnotationIngressRef selects a specific Ingress for backend discovery.
	// Format: "<namespace>/<name>" or just "<name>" (namespace inferred).
	AnnotationIngressRef = "synology.proxy/ingress-ref"

	// AnnotationDestHost overrides the backend hostname / IP.
	AnnotationDestHost = "synology.proxy/destination-host"

	// AnnotationDestPort overrides the backend port.
	AnnotationDestPort = "synology.proxy/destination-port"

	// AnnotationAutoDiscovery is placed on a Namespace to control whether
	// WATCH_NAMESPACE glob-based auto-management applies to resources in that
	// namespace. Set to "false" to disable auto-discovery while still allowing
	// individual resources to opt in via synology.proxy/enabled: "true".
	AnnotationAutoDiscovery = "synology.proxy/auto-discovery"
)
