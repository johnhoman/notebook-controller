package web

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/johnhoman/notebook-controller/apis/v1beta1"
)

const (
	AnnotationKeyDescription = v1beta1.GroupName + "/description"
)

// ListTemplates responds will all the template resources
// that exist in a namespace.
func (app *App) ListTemplates(c *gin.Context) {
	ns := c.Param("namespace")
	if ns == "" {
		// This should never get here because ns
		// is a path parameter
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithCancel(c)
	defer cancel()

	// Users have access to all templates in the current namespace
	// if they have access to the namespace.
	templateList := &v1beta1.TemplateList{}
	// TODO: impersonate users so that authorization will be managed
	//  by the Kubernetes RBAC system
	if err := app.client.List(ctx, templateList, client.InNamespace(ns)); err != nil {
		if errors.IsForbidden(err) {
			// we're using Kubernetes RBAC
			c.AbortWithStatus(http.StatusForbidden)
		}
		app.logger.Error("failed to list templates", zap.Error(err), zap.String("namespace", ns))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	templates := make([]Template, 0)
	for _, item := range templateList.Items {

		opts := make([]TemplateOption, 0, len(item.Spec.Options))
		for _, opt := range item.Spec.Options {
			opts = append(opts, TemplateOption{
				Name:        opt.Name,
				Description: opt.Description,
			})
		}
		templates = append(templates, Template{
			Name:        item.Name,
			Description: item.Annotations[AnnotationKeyDescription],
		})
	}
	c.JSON(http.StatusOK, ListTemplateResponse{Templates: templates})
}

type TemplateOption struct {
	// Unique Name of the template option
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Template struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Options     []TemplateOption `json:"options"`
	Image       string           `json:"image"`
}

type ListTemplateResponse struct {
	Templates []Template `json:"templates"`
}
