// Package v1beta1 defines the api types for the
// notebook provisioner.
// +kubebuilder:object:generate=true
// +groupName=jackhoman.dev
package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

const (
	GroupName = "jackhoman.dev"
	Version   = "v1beta1"
)

var (
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: Version}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&Dag{},
		&DagList{},
		&Execution{},
		&ExecutionList{},
		&Notebook{},
		&NotebookList{},
		&PodDefault{},
		&PodDefaultList{},
		&Revision{},
		&RevisionList{},
		&Template{},
		&TemplateList{},
	)
}
