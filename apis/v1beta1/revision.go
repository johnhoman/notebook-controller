package v1beta1

import (
	"encoding/hex"
	"hash/fnv"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

// RevisionList is a list of revision resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Revision `json:"items"`
}

// Len returns the number of items in the list.
func (in *RevisionList) Len() int {
	return len(in.Items)
}

func (in *RevisionList) Less(i, j int) bool {
	return in.Items[i].Less(&in.Items[j])
}

func (in *RevisionList) Swap(i, j int) {
	in.Items[i], in.Items[j] = in.Items[j], in.Items[i]
}

func (in *RevisionList) Revision(i int) *Revision {
	return &in.Items[i]
}

// ReverseSort sorts the items in reverse order by creation
// timestamp, so the most recently created revision is first. For
// sorting in the other direction, use Sort.
func (in *RevisionList) ReverseSort() {
	sort.Sort(sort.Reverse(in))
}

// Sort sorts the items in order by creation timestamp, so the least
// recently created revision is first. For sorting in the other direction,
// use ReverseSort.
func (in *RevisionList) Sort() {
	sort.Sort(in)
}

// Revision is an immutable snapshot of a workload, such as a
// notebook.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Elected",type=boolean,JSONPath=`.spec.elected`
// +kubebuilder:printcolumn:name="Stopped",type=boolean,JSONPath=`.spec.stopped`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Revision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   RevisionSpec   `json:"spec"`
	Status RevisionStatus `json:"status,omitempty"`
}

func (r *Revision) GetData() []byte {
	return r.Spec.Data.Raw
}

func (r *Revision) AsOwner() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         r.APIVersion,
		Kind:               r.Kind,
		Name:               r.Name,
		UID:                r.UID,
		Controller:         pointer.Bool(true),
		BlockOwnerDeletion: pointer.Bool(true),
	}
}

func (r *Revision) SetData(raw []byte) {
	r.Spec.Data.Raw = raw
}

func (r *Revision) Less(other *Revision) bool {
	return r.CreationTimestamp.Before(&other.CreationTimestamp)
}

func (r *Revision) Elected() bool {
	return r.Spec.Elected
}

func (r *Revision) Elect() {
	r.Spec.Elected = true
}

func (r *Revision) Recall() {
	r.Spec.Elected = false
}

func (r *Revision) Stopped() bool {
	return r.Spec.Stopped
}

func (r *Revision) SetStopped(stopped bool) {
	r.Spec.Stopped = stopped
}

func (r *Revision) Stop() {
	r.SetStopped(true)
}

func (r *Revision) Sum() string {
	hasher := fnv.New32()
	hasher.Write(r.Spec.Data.Raw)
	return hex.EncodeToString(hasher.Sum(nil))
}

func (r *Revision) Hash() string { return r.Sum() }

type RevisionSpec struct {
	// Elected is true is this is the current revision
	// should be created.
	// +kubebuilder:validation:Required
	Elected bool `json:"elected"`
	// Stopped is true if the workload should be stopped. The workload can be both stopped
	// and elected.
	// +kubebuilder:default=true
	Stopped bool `json:"stopped"`
	// Template is an immutable pod template that's a snapshot of the
	// actual runtime workload.
	// +kubebuilder:validation:Required
	Data runtime.RawExtension `json:"snapshot"`
}

type RevisionStatus struct {
	Ready bool `json:"ready"`
}
