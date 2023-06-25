package execution

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/johnhoman/notebook-controller/apis/v1beta1"
	"github.com/johnhoman/notebook-controller/internal/revision"
)

func Setup(mgr manager.Manager) error {

	r := NewReconciler(mgr.GetClient(),
		WithLogger(mgr.GetLogger().WithName("workflow-controller")),
		WithScheme(mgr.GetScheme()),
	)

	return builder.ControllerManagedBy(mgr).
		For(&v1beta1.Execution{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func NewReconciler(client client.Client, opts ...Option) *Reconciler {
	r := &Reconciler{
		client: client,
		scheme: client.Scheme(),
		logger: logr.New(nil),
	}

	for _, f := range opts {
		f(r)
	}
	return r
}

type Option func(r *Reconciler)

func WithLogger(l logr.Logger) Option {
	return func(r *Reconciler) {
		r.logger = l
	}
}

func WithScheme(s *runtime.Scheme) Option {
	return func(r *Reconciler) {
		r.scheme = s
	}
}

type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme
	logger logr.Logger
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	execution := &v1beta1.Execution{}
	if err := r.client.Get(ctx, req.NamespacedName, execution); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}
	if execution.Status.Completed {
		return reconcile.Result{}, nil
	}

	logger := r.logger.WithValues("execution", execution.Name, "namespace", execution.Namespace)

	execution.Status.Tasks = make(map[string]v1beta1.ExecutionTaskStatus)

	dag := &v1beta1.Dag{}
	dag.SetName(execution.Spec.DagRef.Name)
	dag.SetNamespace(execution.Namespace)
	if err := r.client.Get(ctx, client.ObjectKeyFromObject(dag), dag); err != nil {
		logger.Error(err, "failed to get Dag for execution")
		return reconcile.Result{}, err
	}

	tasks := dag.TaskMap()
	completed := sets.New[string]()

	stack := NewStack(execution.MaxConcurrentTasks())
	stack.Push(dag.Entrypoint())
	for !stack.Empty() {
		current := stack.Pop()
		// check if all dependencies are complete, if they are then
		// create a task for the current node. If not, push the
		// current node onto the stack and then all the dependencies
		// onto the stack.

		if !completed.HasAll(current.Dependencies...) {
			stack.Push(current)
			for _, dep := range current.Dependencies {
				if !completed.Has(dep) {
					stack.Push(tasks[dep])
				}
			}
			continue
		}

		// if current is completed or has no dependencies
		// create a task for it
		task := &batchv1.Job{}
		task.SetName(execution.Name + "-" + current.Name)
		task.SetNamespace(execution.Namespace)

		// if the current task isn't completed, increment the inProgress counter
		// so, we can control the number of concurrent tasks
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(task), task); err != nil {
			if !apierrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}

			patches := []v1beta1.PodTemplateSpec{RestartPatch()}

			if len(current.Command) > 0 {
				patches = append(patches, CommandPatch(current.Command...))
			}

			// create the task
			pub := revision.NewPublisher(r.client, revision.WithLogger(r.logger), revision.WithPatches(patches...))
			rev, err := pub.Create(ctx, NamespacedTask{
				Namespace: execution.Namespace,
				DagTask:   &current,
			})
			if err != nil {
				return reconcile.Result{}, err
			}
			// maybe switch this to an init container and run a small sidecar to manage
			// inputs and outputs
			spec := corev1.PodTemplateSpec{}
			if err := json.Unmarshal(rev.GetData(), &spec); err != nil {
				logger.Error(err, "failed to unmarshal pod template spec from revision")
				return reconcile.Result{}, errors.Wrap(err, "failed to unmarshal pod template spec")
			}

			task.OwnerReferences = append(task.OwnerReferences, execution.AsOwner())
			task.Spec = batchv1.JobSpec{
				Template:     spec,
				BackoffLimit: pointer.Int32(0),
				Completions:  pointer.Int32(1),
			}

			if err := r.client.Create(ctx, task); err != nil {
				return reconcile.Result{}, err
			}
		}
		execution.SetTaskStatus(current.Name, v1beta1.ExecutionTaskStatus{
			Conditions: task.Status.Conditions,
			Completed:  !task.Status.CompletionTime.IsZero(),
			Succeeded:  task.Status.Succeeded > 0,
		})

		if !task.Status.CompletionTime.IsZero() {
			completed.Insert(current.Name)
		}

		if task.Status.Failed > 0 {
			execution.Status.Completed = true
			execution.Status.Succeeded = false
			return reconcile.Result{}, r.client.Status().Update(ctx, execution)
		}
	}

	execution.Status.Completed = true
	execution.Status.Succeeded = true
	for _, value := range execution.Status.Tasks {
		execution.Status.Completed = execution.Status.Completed && value.Completed
		execution.Status.Succeeded = execution.Status.Succeeded && value.Succeeded
	}

	if !execution.Status.Completed {
		return reconcile.Result{RequeueAfter: time.Second * 10}, r.client.Status().Update(ctx, execution)
	}
	return reconcile.Result{}, r.client.Status().Update(ctx, execution)
}

func NewStack(maxSize int) *Stack {
	return &Stack{
		maxSize: maxSize,
	}
}

type Stack struct {
	tasks   []v1beta1.DagTask
	maxSize int
}

func (s *Stack) Push(task v1beta1.DagTask) {
	if s.maxSize > 0 && len(s.tasks) < s.maxSize {
		s.tasks = append(s.tasks, task)
	}
}

func (s *Stack) Pop() v1beta1.DagTask {
	if len(s.tasks) == 0 {
		return v1beta1.DagTask{}
	}
	task := s.tasks[len(s.tasks)-1]
	s.tasks = s.tasks[:len(s.tasks)-1]
	return task
}

func (s *Stack) Empty() bool {
	return len(s.tasks) == 0
}

type NamespacedTask struct {
	*v1beta1.DagTask
	Namespace string
}

func (task NamespacedTask) GetNamespace() string {
	return task.Namespace
}

func RestartPatch() v1beta1.PodTemplateSpec {
	return v1beta1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}

func CommandPatch(command ...string) v1beta1.PodTemplateSpec {
	return v1beta1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "main",
				Command: command,
			}},
		},
	}
}
