package main

import (
	"github.com/alecthomas/kong"
	"go.uber.org/zap/zapcore"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/johnhoman/notebook-controller/apis/v1beta1"
	"github.com/johnhoman/notebook-controller/controller/execution"
	"github.com/johnhoman/notebook-controller/controller/notebook"
)

var CommandLineArgs struct{}

func main() {
	setupLog := zap.New(zap.UseDevMode(true)).WithName("startup")
	setupLog.Info("starting controller")

	cmd := kong.Parse(&CommandLineArgs)
	cmd.FatalIfErrorf(v1beta1.AddToScheme(scheme.Scheme), "failed to add scheme")

	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{
		Scheme: scheme.Scheme,
		Logger: zap.New(zap.UseDevMode(true), func(o *zap.Options) {
			o.TimeEncoder = zapcore.RFC3339TimeEncoder
		}),
	})

	cmd.FatalIfErrorf(err, "failed to create controller manager")

	cmd.FatalIfErrorf(notebook.Setup(mgr), "failed to setup notebook controller")
	cmd.FatalIfErrorf(execution.Setup(mgr), "failed to setup execution controller")
	setupLog.Info("finished setting up notebook controller")
	setupLog.Info("starting manager")
	cmd.FatalIfErrorf(err, mgr.Start(signals.SetupSignalHandler()), "failed to start controller manager")
}
