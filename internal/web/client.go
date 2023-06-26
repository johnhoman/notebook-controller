package web

import (
	"context"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	_ Client = Fake{}
)

type Client interface {
	client.Writer
	client.Reader
	Scheme() *runtime.Scheme
	RESTMapper() meta.RESTMapper
}

type Fake struct {
	GetFunc         func(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error
	ListFunc        func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
	CreateFunc      func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
	DeleteFunc      func(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
	UpdateFunc      func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
	PatchFunc       func(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error
	DeleteAllOfFunc func(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error
	scheme          *runtime.Scheme
}

func (f Fake) Scheme() *runtime.Scheme {
	if f.scheme == nil {
		return scheme.Scheme
	}
	return f.scheme
}

func (f Fake) RESTMapper() meta.RESTMapper {
	return meta.NewDefaultRESTMapper([]schema.GroupVersion{})
}

func (f Fake) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return f.GetFunc(ctx, key, obj, opts...)
}

func (f Fake) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return f.ListFunc(ctx, list, opts...)
}

func (f Fake) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return f.CreateFunc(ctx, obj, opts...)
}

func (f Fake) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return f.DeleteFunc(ctx, obj, opts...)
}

func (f Fake) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return f.UpdateFunc(ctx, obj, opts...)
}

func (f Fake) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return f.PatchFunc(ctx, obj, patch, opts...)
}

func (f Fake) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return f.DeleteAllOfFunc(ctx, obj, opts...)
}
