package v1beta1

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExecutionList is a list of Execution resources
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ExecutionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Execution `json:"items,omitempty"`
}

// An Execution is a job that runs a Dag.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
type Execution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	// Spec is the specification of the Execution.
	Spec ExecutionSpec `json:"spec"`
	// Status is the current status of the Execution. That execution
	// status will be updated as tasks are complete.
	// +optional
	Status ExecutionStatus `json:"status,omitempty"`
}

// MaxConcurrentTasks returns the maximum number of tasks that can be run concurrently.
// If unspecified, the default is 20.
func (e *Execution) MaxConcurrentTasks() int {
	if e.Spec.Parallelism == 0 {
		return 20
	}
	return e.Spec.Parallelism
}

func (e *Execution) SetTaskStatus(task string, status ExecutionTaskStatus) {
	if e.Status.Tasks == nil {
		e.Status.Tasks = make(map[string]ExecutionTaskStatus)
	}
	e.Status.Tasks[task] = status
}

func (e *Execution) AsOwner() metav1.OwnerReference {
	b := true
	return metav1.OwnerReference{
		APIVersion:         e.APIVersion,
		Kind:               e.Kind,
		Name:               e.Name,
		UID:                e.UID,
		Controller:         &b,
		BlockOwnerDeletion: &b,
	}
}

type ExecutionSpec struct {
	// DagRef is the name of the Dag to execute.
	// +kubebuilder:validation:Required
	DagRef corev1.LocalObjectReference `json:"dagRef"`
	// Parallelism is the number of jobs to run in parallel.
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=20
	// +kubebuilder:default=10
	// +kubebuilder:validation:Optional
	Parallelism int `json:"parallelism"`
}

type ExecutionStatus struct {
	// Tasks is a map of task names to their current status.
	Tasks map[string]ExecutionTaskStatus `json:"tasks"`
	// Completed is true when all tasks have completed.
	Completed bool `json:"completed"`
	// Succeeded is true when all tasks have completed successfully.
	Succeeded bool `json:"succeeded"`
}

type ExecutionTaskStatus struct {
	// Phase is the current phase of the task.
	Conditions []batchv1.JobCondition `json:"conditions,omitempty"`
	// Completed when all tasks have completed or a single task fails.
	Completed bool `json:"completed"`
	// Succeeded is true when all tasks have completed successfully.
	Succeeded bool `json:"succeeded"`
}
