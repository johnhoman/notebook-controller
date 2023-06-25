package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// A DagList list is a list of Dag resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DagList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Dag `json:"items,omitempty"`
}

// A Dag is a spec for a directed acyclic graph.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Dag struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec DagSpec `json:"spec"`
}

// Entrypoint returns the entrypoint task in the Dag
func (dag *Dag) Entrypoint() DagTask {
	tm := dag.TaskMap()
	return tm[dag.Spec.Entrypoint]
}

// TaskMap returns a mapping of task names to task specs
func (dag *Dag) TaskMap() map[string]DagTask {
	tasks := make(map[string]DagTask)
	for _, task := range dag.Spec.Tasks {
		tasks[task.Name] = task
	}
	return tasks
}

type DagSpec struct {
	// The Entrypoint is the first task in the DAG
	Entrypoint string `json:"entrypoint"`
	// Tasks are the tasks in the DAG
	Tasks []DagTask `json:"tasks"`
}

type DagTask struct {
	// Name is the name of the task. The name is required to create
	// dependencies
	Name string `json:"name"`
	// Template is the name of the template to use for the DagTask's job.
	Template TemplateReference `json:"templateRef"`
	// Command is the command to run in the DagTask's job. If Command is
	// omitted, the command from the Template will be used.
	Command []string `json:"command,omitempty"`
	// Options are the names of PodDefaults that should be merged into
	// the task's pod template. The PodDefaults must be options in the template
	// to be used.
	Options []corev1.LocalObjectReference `json:"options,omitempty"`
	// Dependencies are the names of other tasks that must complete
	// before this task can start.
	Dependencies []string `json:"dependencies,omitempty"`
	// Resources are resources requested for the task, such as
	// memory, and cpu. If Resources is omitted, the defaults
	// from the Template will be used
	// +kubebuilder:validation:Optional
	Resources corev1.ResourceList `json:"resources,omitempty"`
}

func (in *DagTask) HasDependencies() bool {
	return len(in.Dependencies) > 0
}

func (in *DagTask) GetName() string {
	return in.Name
}

func (in *DagTask) TemplateRef() types.NamespacedName {
	return types.NamespacedName{
		Name:      in.Template.Name,
		Namespace: in.Template.Namespace,
	}
}

func (in *DagTask) HistoryLimit() int {
	return 1
}

func (in *DagTask) ResourceRequests() corev1.ResourceList {
	return in.Resources
}

func (in *DagTask) ElectedOptions() []corev1.LocalObjectReference {
	return in.Options
}

func (in *DagTask) UpdatePolicy() string {
	return UpdatePolicyIgnore
}
