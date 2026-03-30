package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SynologyProxyRuleSpec defines the desired state of a reverse proxy rule on Synology DSM.
type SynologyProxyRuleSpec struct {
	// SourceHost is the primary public FQDN that the reverse proxy will listen on (frontend).
	// +kubebuilder:validation:Required
	SourceHost string `json:"sourceHost"`

	// AdditionalSourceHosts lists extra public FQDNs that should each get their own
	// DSM reverse proxy record pointing at the same backend.
	// Each additional host creates a separate record in Synology DSM with its own
	// certificate assigned automatically.
	// +optional
	AdditionalSourceHosts []string `json:"additionalSourceHosts,omitempty"`

	// SourcePort is the HTTPS port the reverse proxy listens on. Defaults to 443.
	// +kubebuilder:default=443
	// +optional
	SourcePort int `json:"sourcePort,omitempty"`

	// DestinationHost is the backend IP or hostname to proxy traffic to.
	// When empty, the operator will auto-discover it from ServiceRef or IngressRef.
	// +optional
	DestinationHost string `json:"destinationHost,omitempty"`

	// DestinationPort is the backend port to proxy traffic to.
	// When 0, the operator will auto-discover it from ServiceRef or IngressRef.
	// +optional
	DestinationPort int `json:"destinationPort,omitempty"`

	// DestinationProtocol is the backend protocol: "http" or "https". Defaults to "http".
	// +kubebuilder:default=http
	// +kubebuilder:validation:Enum=http;https
	// +optional
	DestinationProtocol string `json:"destinationProtocol,omitempty"`

	// ACLProfile is the name of the Synology Access Control Profile to apply.
	// Uses the operator-level default when empty.
	// +optional
	ACLProfile string `json:"aclProfile,omitempty"`

	// AssignCertificate controls whether the operator automatically assigns
	// the best matching wildcard/SAN certificate from Synology. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	AssignCertificate *bool `json:"assignCertificate,omitempty"`

	// CustomHeaders are HTTP headers added to every proxied request.
	// When nil, the operator injects the default WebSocket upgrade headers.
	// +optional
	CustomHeaders []CustomHeader `json:"customHeaders,omitempty"`

	// ServiceRef points to a LoadBalancer Service whose ExternalIP/port will be used
	// as the backend when DestinationHost / DestinationPort are not set.
	// +optional
	ServiceRef *ObjectRef `json:"serviceRef,omitempty"`

	// IngressRef points to an Ingress whose status IP will be used as the backend
	// when DestinationHost / DestinationPort are not set and ServiceRef is absent.
	// +optional
	IngressRef *ObjectRef `json:"ingressRef,omitempty"`

	// Timeouts configures per-proxy TCP/HTTP timeouts (seconds). Defaults to 60 each.
	// +optional
	Timeouts *ProxyTimeouts `json:"timeouts,omitempty"`

	// Description is the human-readable label stored on the Synology record.
	// Defaults to the SynologyProxyRule resource name.
	// +optional
	Description string `json:"description,omitempty"`

	// ManagedByApp records the ArgoCD Application name that owns this rule.
	// Set automatically by the ArgoCD watcher; do not set manually.
	// +optional
	ManagedByApp string `json:"managedByApp,omitempty"`
}

// CustomHeader is an HTTP request header injected by the reverse proxy.
type CustomHeader struct {
	// Name is the header name.
	Name string `json:"name"`
	// Value is the header value (nginx variables are accepted, e.g. $http_upgrade).
	Value string `json:"value"`
}

// ObjectRef is a namespace/name pointer to a Kubernetes object.
type ObjectRef struct {
	// Name of the referenced object.
	Name string `json:"name"`
	// Namespace of the referenced object. Defaults to the SynologyProxyRule namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ProxyTimeouts holds the three Synology proxy timeout values (in seconds).
type ProxyTimeouts struct {
	// Connect timeout in seconds. Defaults to 60.
	// +kubebuilder:default=60
	Connect int `json:"connect,omitempty"`
	// Read timeout in seconds. Defaults to 60.
	// +kubebuilder:default=60
	Read int `json:"read,omitempty"`
	// Send timeout in seconds. Defaults to 60.
	// +kubebuilder:default=60
	Send int `json:"send,omitempty"`
}

// ManagedRecord tracks a single DSM reverse proxy record created by the operator.
type ManagedRecord struct {
	// Description is the DSM record description used as the unique lookup key.
	Description string `json:"description"`
	// UUID is the DSM record UUID.
	UUID string `json:"uuid"`
	// SourceHost is the public FQDN this record fronts.
	SourceHost string `json:"sourceHost"`
}

// SynologyProxyRuleStatus defines the observed state of SynologyProxyRule.
type SynologyProxyRuleStatus struct {
	// ManagedRecords lists all DSM proxy records currently managed by this rule
	// (one per sourceHost / additionalSourceHosts entry).
	// +optional
	ManagedRecords []ManagedRecord `json:"managedRecords,omitempty"`

	// Synced indicates whether the last reconciliation succeeded.
	Synced bool `json:"synced"`

	// LastSyncTime is the timestamp of the most recent successful sync.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// ResolvedDestinationHost is the backend IP/hostname resolved during auto-discovery.
	// +optional
	ResolvedDestinationHost string `json:"resolvedDestinationHost,omitempty"`

	// ResolvedDestinationPort is the backend port resolved during auto-discovery.
	// +optional
	ResolvedDestinationPort int `json:"resolvedDestinationPort,omitempty"`

	// Conditions holds standard Kubernetes status conditions.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// Condition type constants.
const (
	// ConditionSynced is true when the proxy rule is in sync with Synology DSM.
	ConditionSynced = "Synced"
	// ConditionReady is true when backend discovery succeeded and the rule is active.
	ConditionReady = "Ready"

	// ReasonSyncSuccess is used when the Synology API call succeeded.
	ReasonSyncSuccess = "SyncSuccess"
	// ReasonSyncFailed is used when the Synology API call failed.
	ReasonSyncFailed = "SyncFailed"
	// ReasonDiscoveryFailed is used when backend service/ingress lookup failed.
	ReasonDiscoveryFailed = "DiscoveryFailed"
	// ReasonDeleting is used during finalizer processing.
	ReasonDeleting = "Deleting"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=spr
// +kubebuilder:printcolumn:name="Source Host",type=string,JSONPath=`.spec.sourceHost`
// +kubebuilder:printcolumn:name="Destination",type=string,JSONPath=`.status.resolvedDestinationHost`
// +kubebuilder:printcolumn:name="Synced",type=boolean,JSONPath=`.status.synced`
// +kubebuilder:printcolumn:name="Records",type=integer,JSONPath=`.status.managedRecords`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SynologyProxyRule is the Schema for the synologyproxyrules API.
// Each resource represents one reverse proxy entry managed in Synology DSM.
type SynologyProxyRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SynologyProxyRuleSpec   `json:"spec,omitempty"`
	Status SynologyProxyRuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SynologyProxyRuleList contains a list of SynologyProxyRule.
type SynologyProxyRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SynologyProxyRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SynologyProxyRule{}, &SynologyProxyRuleList{})
}
