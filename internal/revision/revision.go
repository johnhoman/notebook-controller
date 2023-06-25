package revision

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/johnhoman/notebook-controller/apis/v1beta1"
)

const (
	ErrReferencedOptionNotFound = "the referenced option was not found in the template spec"
)

var (
	LabelKeyTemplate = fmt.Sprintf("%s/template", v1beta1.GroupName)
	LabelKeyName     = fmt.Sprintf("%s/name", v1beta1.GroupName)
)

// A Referrer is a resource that references a template
type Referrer interface {
	// GetName returns the name of the resource
	GetName() string
	// GetNamespace returns the namespace of the resource
	GetNamespace() string
	// TemplateRef returns the name and namespace of the template
	TemplateRef() types.NamespacedName
	// HistoryLimit returns the number of revisions to keep around
	// for this resource
	HistoryLimit() int
	// ResourceRequests returns the resource requests for this
	// resource
	ResourceRequests() corev1.ResourceList
	// ElectedOptions returns the elected options for this resource
	// if any. Elected options must exist in the template. If additional
	// patches are required, they should be applied by webhook.
	ElectedOptions() []corev1.LocalObjectReference
	// UpdatePolicy returns the update policy for this resource. The UpdatePolicy
	// can either be "Auto", or "Ignore".
	UpdatePolicy() string
}

type Option func(p *Publisher)

func WithLogger(logger logr.Logger) Option {
	return func(p *Publisher) {
		p.logger = logger
	}
}

func WithPatches(patches ...v1beta1.PodTemplateSpec) Option {
	return func(p *Publisher) {
		p.patches = append(p.patches, patches...)
	}
}

func NewPublisher(client client.Client, opts ...Option) *Publisher {
	p := &Publisher{
		client: client,
		logger: logr.New(nil),
		scheme: client.Scheme(),
	}
	for _, f := range opts {
		f(p)
	}
	return p
}

// Publisher creates snapshots of templates with optional overrides
// of patches to the template. The Publisher is based on two resources,
// the template, and the resource referencing the template.
type Publisher struct {
	client client.Client
	scheme *runtime.Scheme
	logger logr.Logger

	patches []v1beta1.PodTemplateSpec
}

func (r *Publisher) SetScheme(scheme *runtime.Scheme) {
	r.scheme = scheme
}

func (r *Publisher) SetLogger(logger logr.Logger) {
	r.logger = logger
}

func (r *Publisher) revisionLabelSet(impl Referrer) map[string]string {
	return map[string]string{
		LabelKeyName: impl.GetName(),
	}
}

// TrimRevisions cleans up old revisions for a given Referrer. A Revision is
// considered old and eligible for deletion if it is not elected and is not
// within the history limit.
func (r *Publisher) TrimRevisions(ctx context.Context, impl Referrer) error {
	revList, err := r.List(ctx, impl)
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(revList))
	for k := impl.HistoryLimit(); k < revList.Len(); k++ {
		// Don't delete elected revisions, they are still in use.
		// This will cause the history limit to be exceeded, but
		// by 1 at one, which seems fine.
		if revList.Revision(k).Elected() {
			continue
		}
		if err := r.client.Delete(ctx, revList.Revision(k)); err != nil {
			return err
		}
	}

	return nil
}

// List the revisions for a given referrer.
func (r *Publisher) List(ctx context.Context, impl Referrer) (*v1beta1.RevisionList, error) {

	opts := []client.ListOption{
		client.InNamespace(impl.GetNamespace()),
		client.MatchingLabels(r.revisionLabelSet(impl)),
	}

	revList := &v1beta1.RevisionList{}
	return revList, r.client.List(ctx, revList, opts...)
}

// Latest returns the latest publisher for the given Template and Referrer.
func (r *Publisher) Latest(ctx context.Context, impl Referrer) (*v1beta1.Revision, error) {
	revList, err := r.List(ctx, impl)
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(revList))
	if revList.Len() == 0 {
		return nil, nil
	}
	return revList.Revision(0), nil
}

// Elected returns the elected publisher for the given Referrer. If
// there is no elected publisher, nil is returned.
func (r *Publisher) Elected(ctx context.Context, impl Referrer) (*v1beta1.Revision, error) {
	revList, err := r.List(ctx, impl)
	if err != nil {
		return nil, err
	}
	for k := 0; k < revList.Len(); k++ {
		rev := revList.Revision(k)
		if rev.Elected() {
			return rev, nil
		}
	}
	return nil, nil
}

// ElectRevision sets a revision as the elected revision. The elected revision
// will either be the latest revision, or the revision that's already elected.
func (r *Publisher) ElectRevision(ctx context.Context, impl Referrer) (*v1beta1.Revision, error) {
	elected, err := r.Elected(ctx, impl)
	if err != nil {
		return nil, err
	}
	if elected == nil || impl.UpdatePolicy() == v1beta1.UpdatePolicyAuto {
		// if there is no elected revision or the update policy is Auto set
		// the latest revision as the elected revision.
		var latest *v1beta1.Revision
		if impl.UpdatePolicy() == v1beta1.UpdatePolicyAuto {
			latest, err = r.Create(ctx, impl)
		} else {
			latest, err = r.Latest(ctx, impl)
		}
		if err != nil {
			return nil, err
		}

		if latest == nil {
			// if the latest revision is nil, then no revisions exist, so
			// create it now.
			latest, err = r.Create(ctx, impl)
			if err != nil {
				return nil, err
			}
		}

		if elected != nil {
			patch := client.MergeFrom(elected.DeepCopyObject().(client.Object))
			elected.Recall()
			if err := r.client.Patch(ctx, elected, patch); err != nil {
				return nil, err
			}
		}

		patch := client.MergeFrom(latest.DeepCopyObject().(client.Object))
		latest.Elect()
		return latest, r.client.Patch(ctx, latest, patch)
	}
	return elected, nil
}

// Create a new publisher for the given Referrer. The publisher is
// only created if the template has changed since the last publisher.
func (r *Publisher) Create(ctx context.Context, impl Referrer) (*v1beta1.Revision, error) {

	template := &v1beta1.Template{}
	if err := r.client.Get(ctx, impl.TemplateRef(), template); err != nil {
		return nil, err
	}

	logger := r.logger.WithValues("Namespace", impl.GetNamespace())

	spec := template.PodTemplateSpec()

	// opts are local to the template namespace, not the impl namespace.
	opts := sets.New[corev1.LocalObjectReference]()
	for _, opt := range template.Options() {
		opts.Insert(corev1.LocalObjectReference{Name: opt.Name})
	}
	// deps are local to the template namespace, not the impl namespace.
	deps := make([]v1beta1.LocalObjectReference, 0)

	for i, opt := range template.Required() {

		pd := &v1beta1.PodDefault{}
		pd.SetName(opt.Name)
		pd.SetNamespace(template.GetNamespace())

		if err := r.client.Get(ctx, client.ObjectKeyFromObject(pd), pd); err != nil {
			logger.Error(err, fmt.Sprintf("failed to get template option %q (pos %d)", opt.Name, i))
			return nil, err
		}

		if err := spec.StrategicMergeFrom(pd.PodTemplateSpec()); err != nil {
			logger.Error(err, "failed to merge template option", "option", opt.Name, "pos", i)
			return nil, err
		}

		for _, item := range pd.Dependencies() {
			deps = append(deps, item)
		}
	}

	for i, opt := range impl.ElectedOptions() {
		if !opts.Has(opt) {
			return nil, errors.Errorf("%s: %s", ErrReferencedOptionNotFound, opt.Name)
		}

		pd := &v1beta1.PodDefault{}
		pd.SetName(opt.Name)
		pd.SetNamespace(template.GetNamespace())

		if err := r.client.Get(ctx, client.ObjectKeyFromObject(pd), pd); err != nil {
			logger.Error(err, fmt.Sprintf("failed to get template option %q (pos %d)", opt.Name, i))
			return nil, err
		}

		if err := spec.StrategicMergeFrom(pd.PodTemplateSpec()); err != nil {
			logger.Error(err, "failed to merge template option", "option", opt.Name, "pos", i)
			return nil, err
		}

		for _, item := range pd.Dependencies() {
			deps = append(deps, item)
		}
	}

	for _, patch := range r.patches {
		if err := spec.StrategicMergeFrom(patch); err != nil {
			logger.Error(err, "failed to merge patch")
			return nil, err
		}
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		logger.Error(err, "failed to marshal revision spec")
		return nil, err
	}

	rev := &v1beta1.Revision{}
	rev.SetData(data)
	rev.SetLabels(r.revisionLabelSet(impl))
	rev.SetAnnotations(map[string]string{LabelKeyTemplate: impl.TemplateRef().String()})
	rev.SetName(impl.GetName() + "-" + rev.Hash())
	rev.SetNamespace(impl.GetNamespace())
	// if the publisher is created successfully, copy all the dependencies
	// and set the parent as the publisher
	if err := r.client.Create(ctx, rev); client.IgnoreAlreadyExists(err) != nil {
		logger.Error(err, "failed to create revision")
		return nil, err
	}

	for _, dep := range deps {
		o, err := r.scheme.New(dep.GroupVersionKind())
		if err != nil {
			obj := &unstructured.Unstructured{}
			obj.SetGroupVersionKind(dep.GroupVersionKind())
			o = obj
		}
		obj := o.(client.Object)
		obj.SetNamespace(template.GetNamespace())
		if err := r.client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			logger.Error(err, "cannot create instance of dependency", "dependency", dep.Name)
			return nil, err
		}
		obj.SetOwnerReferences(append(obj.GetOwnerReferences(), rev.AsOwner()))
		obj.SetName(rev.GetName())
		obj.SetNamespace(rev.GetNamespace())

		// reset all metadata
		obj.SetCreationTimestamp(metav1.Time{})
		obj.SetDeletionTimestamp(nil)
		obj.SetFinalizers(nil)
		obj.SetResourceVersion("")
		obj.SetSelfLink("")
		obj.SetGenerateName("")
		obj.SetDeletionGracePeriodSeconds(nil)
		obj.SetManagedFields(nil)
		if err := r.client.Create(ctx, obj); client.IgnoreAlreadyExists(err) != nil {
			logger.Error(err, "failed to create dependency", "dependency", dep.Name)
			return nil, err
		}
	}
	return rev, r.TrimRevisions(ctx, impl)
}

func createStrategicMergePatch(from, to v1beta1.PodTemplateSpec) ([]byte, error) {
	original, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	if err != nil {
		return nil, err
	}
	modified, err := runtime.DefaultUnstructuredConverter.ToUnstructured(to)
	if err != nil {
		return nil, err
	}
	patch, err := strategicpatch.CreateTwoWayMergeMapPatch(original, modified, v1beta1.PodTemplateSpec{})
	if err != nil {
		return nil, err
	}
	return json.Marshal(patch)

}
