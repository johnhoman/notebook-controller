package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

type ObjectMeta struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type PodTemplateSpec struct {
	ObjectMeta `json:"metadata,omitempty"`
	Spec       corev1.PodSpec `json:"spec"`
}

func (in *PodTemplateSpec) StrategicMergeFrom(other PodTemplateSpec) error {
	into, err := runtime.DefaultUnstructuredConverter.ToUnstructured(in)
	if err != nil {
		return err
	}

	from, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&other)
	if err != nil {
		return err
	}
	// containers is a required field in a PodSpec, so if it's not part of the patch
	// the conversion to unstructured will have a nil spec.containers field, which
	// will clear out the containers field in the merge.  We don't want that.
	if spec := from["spec"].(map[string]any); spec["containers"] == nil {
		delete(spec, "containers")
	}

	into, err = strategicpatch.StrategicMergeMapPatch(into, from, PodTemplateSpec{})
	if err != nil {
		return err
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(into, in)
}

func (in *PodTemplateSpec) PodTemplateSpec() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      in.ObjectMeta.Labels,
			Annotations: in.ObjectMeta.Annotations,
		},
		Spec: in.Spec,
	}
}

// Orphan is a type that can be adopted
// +k8s:deepcopy-gen=false
type Orphan interface {
	SetOwnerReferences([]metav1.OwnerReference)
	GetOwnerReferences() []metav1.OwnerReference
}
