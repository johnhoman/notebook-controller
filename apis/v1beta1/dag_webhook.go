package v1beta1

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	ErrDuplicateTaskName = "duplicate task name"
)

var (
	_ admission.Validator = &Dag{}
	_ admission.Defaulter = &Dag{}
)

func (dag *Dag) Default() { return }

func (dag *Dag) ValidateCreate() (warnings admission.Warnings, err error) {
	return nil, ValidateDag(dag)
}

func (dag *Dag) ValidateUpdate(old runtime.Object) (warnings admission.Warnings, err error) {
	return nil, ValidateDag(dag)
}

func (dag *Dag) ValidateDelete() (warnings admission.Warnings, err error) {
	return nil, ValidateDag(dag)
}

func ValidateDag(dag *Dag) error {
	m := make(map[string]bool)
	for _, task := range dag.Spec.Tasks {
		if m[task.Name] {
			return errors.Errorf("%s: task name %q is duplicated", ErrDuplicateTaskName, task.Name)
		}
		m[task.Name] = true
	}
	return nil
}
