package notebook

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/johnhoman/notebook-controller/apis/v1beta1"
	"github.com/johnhoman/notebook-controller/internal/revision"
)

var (
	_ reconcile.Reconciler = &Reconciler{}

	LabelKeyNotebookName = fmt.Sprintf("%s/notebook-name", v1beta1.GroupName)
	AnnotationKeyOwner   = fmt.Sprintf("%s/owner", v1beta1.GroupName)
)

// Setup adds the Notebook controller to manager.Manager.
func Setup(mgr manager.Manager) error {
	r := NewReconciler(mgr.GetClient(),
		WithLogger(mgr.GetLogger().WithName("notebook-controller")),
		WithScheme(mgr.GetScheme()),
	)

	return builder.ControllerManagedBy(mgr).
		For(&v1beta1.Notebook{}).
		Owns(&v1beta1.Revision{}).
		Owns(&corev1.Pod{}).
		Watches(&v1beta1.Template{}, EnqueueRequestFromTemplate(mgr.GetCache(), mgr.GetLogger())).
		Complete(r)
}

type Option func(r *Reconciler)

func WithLogger(logger logr.Logger) Option {
	return func(r *Reconciler) {
		r.logger = logger
	}
}

// WithScheme sets the scheme on the Reconciler. If the scheme is not
// provided, the client's scheme will be used.
func WithScheme(scheme *runtime.Scheme) Option {
	return func(r *Reconciler) {
		r.scheme = scheme
	}
}

// NewReconciler returns a new Reconciler with default options
// set as well as any options provided. If the provided options
// conflict with the defaults, the provided options will take
// precedence.
func NewReconciler(cli client.Client, opts ...Option) *Reconciler {
	r := &Reconciler{
		client: cli,
		scheme: cli.Scheme(),
		logger: logr.New(nil),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

type Reconciler struct {
	client client.Client
	scheme *runtime.Scheme
	logger logr.Logger

	// Namespace is the namespace in which the controller is running.
	// Templates that exist in this namespace can be referenced by notebooks
	// in other namespace. Dependencies that exist in this namespace will
	// be copied to other namespaces when referenced by a notebook.
	namespace string
}

// Reconcile creates a NotebookRevision from a Notebook and Template spec. Notebook
// revisions are created when there is a change in the Template, and sometimes a change
// in the Notebook spec, depending on what options from the template have been selected.
// Changing resource requests and limits in the Notebook spec will not trigger a new revision.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {

	nb := &v1beta1.Notebook{}
	if err := r.client.Get(ctx, req.NamespacedName, nb); err != nil {
		r.logger.Info("unable to fetch Notebook", "error", err)
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	pod := &corev1.Pod{}
	pod.SetName(nb.Name)
	pod.SetNamespace(nb.Namespace)

	if nb.Stopped() {
		r.logger.Info("notebook is stopped")
		if err := r.client.Delete(ctx, pod); client.IgnoreNotFound(err) != nil {
			r.logger.Info("unable to delete Pod", "error", err)
			return reconcile.Result{}, err
		}
		patch := client.MergeFrom(nb.DeepCopy())
		nb.Status.Phase = v1beta1.NotebookPhaseStopped
		nb.Status.Conditions = pod.Status.Conditions
		return reconcile.Result{}, r.client.Status().Patch(ctx, nb, patch)
	}

	if nb.Spec.TemplateRef.Namespace != r.namespace && nb.Spec.TemplateRef.Namespace != nb.Namespace {
		r.logger.Info("templateRef namespace must be the same as the namespace of the Notebook or the system namespace")
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, func() error {
		pub := revision.NewPublisher(r.client, revision.WithLogger(r.logger))

		if err := r.client.Get(ctx, client.ObjectKeyFromObject(pod), pod); err != nil {
			if !errors.IsNotFound(err) {
				r.logger.Info("unable to fetch Pod", "error", err)
				return err
			}

			// When the pod doesn't exist, we need to create it from the revision.
			elected, err := pub.Elected(ctx, nb)
			if err != nil {
				r.logger.Info("unable to elect revision", "error", err)
				return err
			}
			if elected == nil {
				r.logger.Info("revision not elected")
				return nil
			}
			data := elected.GetData()
			spec := v1beta1.PodTemplateSpec{}
			if err := json.Unmarshal(data, &spec); err != nil {
				r.logger.Info("unable to unmarshal revision", "error", err)
				return err
			}
			if spec.Labels == nil {
				spec.Labels = make(map[string]string)
			}
			if spec.Annotations == nil {
				spec.Annotations = make(map[string]string)
			}

			spec.Labels[LabelKeyNotebookName] = nb.Name
			spec.Annotations[AnnotationKeyOwner] = nb.Spec.Owner.Name

			pod.Spec = spec.Spec
			pod.Labels = spec.Labels
			pod.Annotations = spec.Annotations
			pod.OwnerReferences = append(pod.OwnerReferences, nb.AsOwner())
			if err := r.client.Create(ctx, pod); err != nil {
				return err
			}
		}

		revList, err := pub.List(ctx, nb)
		if err != nil {
			return err
		}

		patch := client.MergeFrom(nb.DeepCopy())
		nb.Status.Phase = pod.Status.Phase
		nb.Status.Conditions = pod.Status.Conditions
		nb.Status.Revisions = make([]v1beta1.NotebookRevision, revList.Len())
		for k := 0; k < revList.Len(); k++ {
			nb.Status.Revisions[k].Name = revList.Revision(k).GetName()
			nb.Status.Revisions[k].CreatedAt = revList.Revision(k).GetCreationTimestamp()
			nb.Status.Revisions[k].Elected = revList.Revision(k).Elected()
		}
		sort.Slice(nb.Status.Revisions, func(i, j int) bool {
			return nb.Status.Revisions[j].CreatedAt.Before(&nb.Status.Revisions[i].CreatedAt)
		})
		return r.client.Status().Patch(ctx, nb, patch)
	}()
}

func EnqueueRequestFromTemplate(cache cache.Cache, logger logr.Logger) handler.EventHandler {
	err := cache.IndexField(context.Background(), &v1beta1.Notebook{}, "spec.templateRef.name", func(o client.Object) []string {
		nb, ok := o.(*v1beta1.Notebook)
		if !ok {
			return nil
		}
		return []string{nb.Spec.TemplateRef.Name}
	})
	if err != nil {
		panic(err)
	}
	err = cache.IndexField(context.Background(), &v1beta1.Notebook{}, "spec.templateRef.namespace", func(o client.Object) []string {
		nb, ok := o.(*v1beta1.Notebook)
		if !ok {
			return nil
		}
		return []string{nb.Spec.TemplateRef.Namespace}
	})
	if err != nil {
		panic(err)
	}
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		t, ok := obj.(*v1beta1.Template)
		if !ok {
			return nil
		}

		nbList := &v1beta1.NotebookList{}
		if err := cache.List(ctx, nbList, client.MatchingFields{
			"spec.templateRef.name":      t.Name,
			"spec.templateRef.namespace": t.Namespace,
		}); err != nil {
			logger.Info("unable to list notebooks", "error", err)
			return nil
		}
		rv := make([]reconcile.Request, 0, len(nbList.Items))
		for _, nb := range nbList.Items {
			rv = append(rv, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&nb)})
		}
		return rv
	})
}
