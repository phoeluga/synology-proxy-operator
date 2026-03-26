package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SynologyReverseProxySpec defines the desired state of SynologyReverseProxy
type SynologyReverseProxySpec struct {
	// Description is the Synology record name/description
	// +kubebuilder:validation:Required
	Description string `json:"description"`

	// SourceHostname is the frontend FQDN
	// +kubebuilder:validation:Required
	SourceHostname string `json:"sourceHostname"`

	// SourcePort is the frontend port
	// +kubebuilder:default=443
	SourcePort int `json:"sourcePort"`

	// SourceProtocol is the frontend protocol (http or https)
	// +kubebuilder:validation:Enum=http;https
	// +kubebuilder:default=https
	SourceProtocol string `json:"sourceProtocol"`

	// DestHostname is the backend IP or hostname
	// +kubebuilder:validation:Required
	DestHostname string `json:"destHostname"`

	// DestPort is the backend port
	// +kubebuilder:validation:Required
	DestPort int `json:"destPort"`

	// DestProtocol is the backend protocol (http or https)
	// +kubebuilder:validation:Enum=http;https
	// +kubebuilder:default=http
	DestProtocol string `json:"destProtocol"`

	// ACLProfile is the optional ACL profile name to assign
	// +optional
	ACLProfile string `json:"aclProfile,omitempty"`

	// AssignCertificate controls whether to auto-assign a matching wildcard cert
	// +optional
	AssignCertificate bool `json:"assignCertificate,omitempty"`
}

// SynologyReverseProxyStatus defines the observed state of SynologyReverseProxy
type SynologyReverseProxyStatus struct {
	// UUID is the Synology reverse proxy record UUID
	// +optional
	UUID string `json:"uuid,omitempty"`

	// CertID is the certificate ID assigned to this proxy record
	// +optional
	CertID string `json:"certId,omitempty"`

	// Conditions represent the latest available observations of the resource state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=srp
// +kubebuilder:printcolumn:name="Source",type=string,JSONPath=`.spec.sourceHostname`
// +kubebuilder:printcolumn:name="Dest",type=string,JSONPath=`.spec.destHostname`
// +kubebuilder:printcolumn:name="UUID",type=string,JSONPath=`.status.uuid`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SynologyReverseProxy is the Schema for the synologyreverseproxies API
type SynologyReverseProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SynologyReverseProxySpec   `json:"spec,omitempty"`
	Status SynologyReverseProxyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// SynologyReverseProxyList contains a list of SynologyReverseProxy
type SynologyReverseProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SynologyReverseProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SynologyReverseProxy{}, &SynologyReverseProxyList{})
}
