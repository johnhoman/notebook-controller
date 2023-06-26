package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/johnhoman/notebook-controller/apis/v1beta1"
)

func TestApp_ListTemplates(t *testing.T) {

	qt.Assert(t, v1beta1.AddToScheme(scheme.Scheme), qt.IsNil)
	k8s := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	app := &App{client: k8s}

	r := httptest.NewRequest(http.MethodGet, "/api/namespaces/:namespace/templates", nil)
	w := httptest.NewRecorder()

	mux := app.Router(nil)
	mux.ServeHTTP(w, r)

	qt.Assert(t, w.Code, qt.Equals, http.StatusOK)
	qt.Assert(t, w.Body, qt.IsNotNil)

	qt.Assert(t, w.Body.String(), qt.JSONEquals, map[string]any{
		"templates": make([]any, 0),
	})
}
