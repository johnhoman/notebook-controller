package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

const (
	NotebookPhaseRunning = "Running"
	NotebookPhaseStopped = "Stopped"
)

// NotebookList is a list of notebooks
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type NotebookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Notebook `json:"items"`
}

// Notebook is a spec for a notebook resource. A Notebook combined
// with a referenced template will create a NotebookRevision.
// Revisions are the actual runtime workload.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type Notebook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   NotebookSpec   `json:"spec"`
	Status NotebookStatus `json:"status,omitempty"`
}

func (nb *Notebook) AsOwner() metav1.OwnerReference {
	trueVar := true
	return metav1.OwnerReference{
		APIVersion:         nb.APIVersion,
		Kind:               nb.Kind,
		Name:               nb.Name,
		UID:                nb.UID,
		Controller:         &trueVar,
		BlockOwnerDeletion: &trueVar,
	}
}

func (nb *Notebook) Stopped() bool {
	return nb.Spec.Stopped
}

func (nb *Notebook) ElectedOptions() []corev1.LocalObjectReference {
	opts := make([]corev1.LocalObjectReference, len(nb.Spec.Options))
	for _, item := range nb.Spec.Options {
		opts = append(opts, corev1.LocalObjectReference{Name: item})
	}
	return opts
}

func (nb *Notebook) Adopt(obj Orphan) {
	owners := obj.GetOwnerReferences()

	for k := 0; k < len(owners); k++ {
		if owners[k].UID == nb.UID {
			// if this isn't the controller ref, then we need to update it
			// to be the controller ref
			if !pointer.BoolDeref(owners[k].Controller, false) {
				owners[k].Controller = pointer.Bool(true)
				obj.SetOwnerReferences(owners)
			}
			return
		}
	}
	owners = append(owners, *metav1.NewControllerRef(nb, GroupVersion.WithKind("Notebook")))
	obj.SetOwnerReferences(owners)
}

func (nb *Notebook) HistoryLimit() int {
	return nb.Spec.RevisionHistoryLimit
}

func (nb *Notebook) ResourceRequests() corev1.ResourceList {
	return nb.Spec.ResourceRequests
}

func (nb *Notebook) TemplateRef() types.NamespacedName {
	return types.NamespacedName{Name: nb.Spec.TemplateRef.Name, Namespace: nb.Namespace}
}

func (nb *Notebook) HasUpdatePolicy() bool {
	return nb.Spec.UpdatePolicy != nil
}

func (nb *Notebook) UpdatePolicy() string {
	if nb.Spec.UpdatePolicy == nil {
		return ""
	}
	return *nb.Spec.UpdatePolicy
}

func (nb *Notebook) SetUpdatePolicy(up string) {
	nb.Spec.UpdatePolicy = &up
}

type NotebookSpec struct {
	// RevisionHistoryLimit is the number of revisions to keep around
	// after the notebook updates. The oldest revisions will be removed
	// first.
	// +kubebuilder:validation:min=1
	// +kubebuilder:validation:default=3
	RevisionHistoryLimit int `json:"revisionHistoryLimit"`
	// When Stopped is true, the Notebook pod will be removed, so that it
	// is not consuming resources. When Stopped is false, the Notebook will
	// have a single pod running.
	// +kubebuilder:validation:Required
	// +kubebuilder:default=false
	Stopped bool `json:"stopped,omitempty"`
	// The UpdatePolicy specified how to handle changes in the template spec.
	// If the UpdatePolicy is unspecified, the referenced Template update policy
	// will be used. If the UpdatePolicy is set to automatic, the Notebook will
	// be updated during downtime.
	// +kubebuilder:validation:Enum=Auto;Ignore
	UpdatePolicy *string `json:"updatePolicy,omitempty"`
	// ResourceRequests are resources requested for the notebook, such as
	// memory, cpu, and storage. If ResourceRequests is omitted, the defaults
	// from the Template will be used
	// +kubebuilder:validation:Optional
	ResourceRequests corev1.ResourceList `json:"resources,omitempty"`
	// TemplateRef is a reference to a specific template revision. If the
	// template revision isn't specified, the latest template revision will
	// be used.
	// +kubebuilder:validation:Required
	TemplateRef TemplateReference `json:"templateRef"`
	// An Owner is the user that created the Notebook. The Owner is the
	// only user that can update, patch, or delete the Notebook.
	// +kubebuilder:validation:Required
	Owner rbacv1.Subject `json:"owner"`
	// Options are selected template options for the notebook. Chosen
	// Options will be applied directly to the NotebookRevision.
	// +kubebuilder:optional
	Options []string `json:"options,omitempty"`
}

type NotebookRevision struct {
	Name      string      `json:"name"`
	Elected   bool        `json:"elected"`
	CreatedAt metav1.Time `json:"createdAt"`
}

type NotebookStatus struct {
	Conditions []corev1.PodCondition `json:"conditions,omitempty"`
	Phase      corev1.PodPhase       `json:"phase"`
	Revisions  []NotebookRevision    `json:"revisions"`
}

type NotebookCondition struct {
	Type               string      `json:"type"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	Reason             string      `json:"reason"`
	Message            string      `json:"message,omitempty"`
}
