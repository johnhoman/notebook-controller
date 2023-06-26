package execution

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/google/go-cmp/cmp/cmpopts"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/johnhoman/notebook-controller/apis/v1beta1"
)

func TestReconciler_Reconcile(t *testing.T) {
	tasks := []v1beta1.DagTask{{
		Name: "task1",
		Template: v1beta1.TemplateReference{
			Name:      "template1",
			Namespace: "test",
		},
	}}

	dag := newDag("dag1", "test", tasks...)
	template := newTemplate("template1", "test", v1beta1.PodTemplateSpec{})
	execution := &v1beta1.Execution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "execution1",
			Namespace: "test",
		},
		Spec: v1beta1.ExecutionSpec{
			DagRef: corev1.LocalObjectReference{
				Name: "dag1",
			},
		},
	}
	qt.Assert(t, v1beta1.AddToScheme(scheme.Scheme), qt.IsNil)
	k8s := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(dag, template, execution).
		WithStatusSubresource(execution).
		Build()

	ctx := context.Background()

	r := NewReconciler(k8s)
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: "execution1", Namespace: "test",
		},
	}
	res, err := r.Reconcile(ctx, req)
	qt.Assert(t, err, qt.IsNil)
	t.Run("RequeueWhenJobIsNotComplete", func(t *testing.T) {
		qt.Assert(t, res, qt.Equals, reconcile.Result{RequeueAfter: time.Second * 10})
	})
	t.Run("JobIsCreated", func(t *testing.T) {
		want := &batchv1.Job{}
		want.SetName("execution1-task1")
		want.SetNamespace("test")
		want.OwnerReferences = []metav1.OwnerReference{{
			APIVersion:         v1beta1.GroupVersion.String(),
			Kind:               "Execution",
			Name:               "execution1",
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		}}

		tmpl := template.DeepCopy()
		tmpl.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
		want.Spec = batchv1.JobSpec{
			BackoffLimit: pointer.Int32(0),
			Completions:  pointer.Int32(1),
			Template: corev1.PodTemplateSpec{
				Spec: tmpl.Spec.Template.Spec,
			},
		}
		got := &batchv1.Job{}
		qt.Assert(t, k8s.Get(ctx, client.ObjectKeyFromObject(want), got), qt.IsNil)
		qt.Assert(t, got, compareEquals, want)
	})
}

func newTemplate(name, namespace string, podSpec v1beta1.PodTemplateSpec) *v1beta1.Template {
	return &v1beta1.Template{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.TemplateSpec{
			Template: podSpec,
		},
	}
}

func newDag(name, namespace string, tasks ...v1beta1.DagTask) *v1beta1.Dag {
	return &v1beta1.Dag{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.DagSpec{
			Entrypoint: tasks[0].Name,
			Tasks:      tasks,
		},
	}
}

var compareEquals = qt.CmpEquals(
	cmpopts.EquateEmpty(),
	cmpopts.IgnoreFields(metav1.ObjectMeta{}, "ResourceVersion"),
	cmpopts.IgnoreFields(v1beta1.Execution{}, "TypeMeta"),
	cmpopts.IgnoreFields(v1beta1.Template{}, "TypeMeta"),
	cmpopts.IgnoreFields(v1beta1.Notebook{}, "TypeMeta"),
	cmpopts.IgnoreFields(batchv1.Job{}, "TypeMeta"),
)
