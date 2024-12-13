package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterIngressServiceDNSWeight is what defines the links between k8s services and ingresses
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type ClusterIngressServiceDNSWeight struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ClusterIngressServiceDNSWeightSpec `json:"spec"`
}

// ClusterIngressServiceDNSWeightList contains a list of ClusterIngressServiceDNSWeight
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type ClusterIngressServiceDNSWeightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterIngressServiceDNSWeight `json:"items"`
}

// IngressServiceDNSWeightSpec defines the desired state of IngressServiceDNSWeight
type ClusterIngressServiceDNSWeightSpec struct {
	// Weight is the weight of the service
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default:=0
	Weight uint `json:"weight"`

	Identifier      string          `json:"identifier"`
	ServiceSelector ServiceSelector `json:"serviceSelector"`
	IngressSelector IngressSelector `json:"ingressSelector"`
}

// ServiceSelector defines the service selector in a specific namespace
type ServiceSelector struct {
	Namespace            string `json:"namespace"`
	metav1.LabelSelector `json:",inline"`
}

// IngressSelector defines the ingress selector in specific namespaces
type IngressSelector struct {

	// LabelSelector is the list of labels that the ingress must have
	// If not provided, all ingresses will be considered
	metav1.LabelSelector `json:",inline"`

	// Namespaces is the list of namespaces where the ingress selector will be applied
	// If not provided, all namespaces will be considered
	// +kubebuilder:validation:Optional
	Namespaces []string `json:"namespaces,omitempty"`

	// Classes is the list of classes that the ingress must have
	// It must contain at least one class
	// +kubebuilder:validation:minItems=1
	Classes []string `json:"classes"`
}

func init() {
	SchemeBuilder.Register(&ClusterIngressServiceDNSWeight{}, &ClusterIngressServiceDNSWeightList{})
}
