package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodDefaultList is a list of PodDefault resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PodDefaultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PodDefault `json:"items,omitempty"`
}

// A PodDefault is an optional configuration that can be merged
// into workload resources, such as Notebooks.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PodDefault struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec PodDefaultSpec `json:"spec"`
}

func (in *PodDefault) Dependencies() []LocalObjectReference {
	return in.Spec.Dependencies
}

func (in *PodDefault) PodTemplateSpec() PodTemplateSpec {
	return in.Spec.Template
}

// PodDefaultSpec is the spec for a PodDefault resource
type PodDefaultSpec struct {
	Template     PodTemplateSpec        `json:"template"`
	Dependencies []LocalObjectReference `json:"dependencies,omitempty"`
}
