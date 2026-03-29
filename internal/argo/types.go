// Package argo contains minimal type definitions for ArgoCD Application resources.
// Keeping these local avoids importing the full ArgoCD dependency tree.
package argo

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is the ArgoCD Application GroupVersion.
	GroupVersion = schema.GroupVersion{Group: "argoproj.io", Version: "v1alpha1"}

	// SchemeBuilder registers the Application and ApplicationList types.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme is a convenience function for registering types.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&Application{},
		&ApplicationList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

// +kubebuilder:object:root=true

// Application is the minimal ArgoCD Application type used by this operator.
type Application struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApplicationSpec   `json:"spec,omitempty"`
	Status            ApplicationStatus `json:"status,omitempty"`
}

// ApplicationList is a list of Application resources.
type ApplicationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Application `json:"items"`
}

// ApplicationSpec is the subset of ArgoCD Application spec we care about.
type ApplicationSpec struct {
	// Destination is the target cluster/namespace.
	Destination ApplicationDestination `json:"destination,omitempty"`
	// Project is the ArgoCD project name.
	Project string `json:"project,omitempty"`
}

// ApplicationDestination describes where the application will be deployed.
type ApplicationDestination struct {
	// Server is the Kubernetes API server URL.
	Server string `json:"server,omitempty"`
	// Namespace is the target namespace.
	Namespace string `json:"namespace,omitempty"`
	// Name is the named destination cluster.
	Name string `json:"name,omitempty"`
}

// ApplicationStatus contains the observed state.
type ApplicationStatus struct {
	Sync   SyncStatus   `json:"sync,omitempty"`
	Health HealthStatus `json:"health,omitempty"`
}

// SyncStatus describes the sync state of the application.
type SyncStatus struct {
	Status string `json:"status,omitempty"`
}

// HealthStatus describes the health of the application.
type HealthStatus struct {
	Status string `json:"status,omitempty"`
}

// DeepCopyObject implements runtime.Object.
func (in *Application) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := in.DeepCopy()
	return out
}

// DeepCopy creates a deep copy of Application.
func (in *Application) DeepCopy() *Application {
	if in == nil {
		return nil
	}
	out := new(Application)
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	return out
}

// DeepCopyObject implements runtime.Object.
func (in *ApplicationList) DeepCopyObject() runtime.Object {
	if in == nil {
		return nil
	}
	out := new(ApplicationList)
	*out = *in
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		out.Items = make([]Application, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
	return out
}

// DeepCopyInto copies all properties into another Application instance.
func (in *Application) DeepCopyInto(out *Application) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}
