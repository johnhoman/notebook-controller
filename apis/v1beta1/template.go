package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	UpdatePolicyAuto   = "Auto"
	UpdatePolicyIgnore = "Ignore"
)

// TemplateReference is a reference to a specific template revision, or
// the current revision if unspecified. ResourceVersion should be tracked
// for repeatability.
type TemplateReference struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
}

// LocalObjectReference is a reference to an arbitrary object in the
// same namespace as the template.
type LocalObjectReference struct {
	Name       string `json:"name"`
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

func (in *LocalObjectReference) GroupVersionKind() schema.GroupVersionKind {
	gv, _ := schema.ParseGroupVersion(in.APIVersion)
	return gv.WithKind(in.Kind)
}

type TemplateOption struct {
	// Name is an identifier for the optional configuration. The identifier must
	// be unique among optional configuration in a namespace
	Name string `json:"name"`
	// A Description describes what the optional configuration is and what
	// it does
	Description string `json:"description"`
}

type TemplateSpec struct {
	// Dependencies are other resources, such as ConfigMaps, or Secrets
	// that are required by the Template. If the Template is referenced
	// in another namespace, the Dependencies will need to either be copied
	// to the referencing namespace, or the referencing notebook should
	// be rejected
	Dependencies []LocalObjectReference `json:"dependencies,omitempty"`
	// Options are configurations that can be added to the child workload,
	// such as an alternate python package index.
	Options []TemplateOption `json:"options,omitempty"`

	// Required are configurations that MUST be added to the child workload,
	// such as an alternate python package index.
	Required []corev1.LocalObjectReference `json:"required,omitempty"`

	// Template is a full pod spec which serves as the base for a
	// realized notebook. The notebook can optionally override a subset
	// of these parameters, such as resource requests, but in generally
	// the whole spec will be copied to
	Template PodTemplateSpec `json:"template"`
}

// Template is a skeleton for a notebook, similar to PodDefaults,
// but the starting point instead of an addon. It allows an ops
// team to configure user's Pods before they are created rather
// than after.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec TemplateSpec `json:"spec"`
}

func (in *Template) Dependencies() []LocalObjectReference {
	return in.Spec.Dependencies
}

func (in *Template) Options() []TemplateOption {
	return in.Spec.Options
}

func (in *Template) Required() []corev1.LocalObjectReference {
	return in.Spec.Required
}

func (in *Template) PodTemplateSpec() *PodTemplateSpec {
	return &PodTemplateSpec{
		ObjectMeta: ObjectMeta{
			Annotations: in.Spec.Template.Annotations,
			Labels:      in.Spec.Template.Labels,
		},
		Spec: in.Spec.Template.Spec,
	}
}

func (in *Template) SetResourceRequests(req corev1.ResourceList) {
	if len(in.Spec.Template.Spec.Containers) == 0 {
		// nothing to do, there are no containers
		return
	}
	container := &in.Spec.Template.Spec.Containers[0]
	if container.Resources.Requests == nil {
		container.Resources.Requests = req
		return
	}
	for k, v := range req {
		container.Resources.Requests[k] = v
	}
	return
}

// TemplateList is a list of templates
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Template `json:"items,omitempty"`
}
