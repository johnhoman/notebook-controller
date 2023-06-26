package web

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
)

type App struct {
	client Client

	logger *zap.Logger

	userIDHeaderKey    string
	userIDHeaderPrefix string
	groupHeaderKey     string
	groupHeaderPrefix  string
}

func (app *App) Router(r *gin.Engine) http.Handler {
	if r == nil {
		r = gin.Default()
	}
	r.GET("/api/namespaces/:namespace/templates", app.ListTemplates)
	return r
}
