package cmd

import (
	"fmt"

	"github.com/mohammadne/caas-operator/internal/config"
	"github.com/mohammadne/caas-operator/pkg/logger"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"

	appsv1alpha1 "github.com/mohammadne/caas-operator/internal/api/v1alpha1"
	"github.com/mohammadne/caas-operator/internal/cloudflare"
	"github.com/mohammadne/caas-operator/internal/controllers"
	//+kubebuilder:scaffold:imports
)

type Server struct{}

func (cmd Server) Command() *cobra.Command {
	run := func(_ *cobra.Command, _ []string) {
		cmd.main(config.Load(true))
	}

	return &cobra.Command{
		Use:   "server",
		Short: "run controller-manager",
		Run:   run,
	}
}

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(appsv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func (cmd *Server) main(cfg *config.Config) {
	logger := logger.NewZap(cfg.Logger)

	manager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     fmt.Sprintf(":%d", cfg.MetricsPort),
		HealthProbeBindAddress: fmt.Sprintf(":%d", cfg.ProbePort),
		Port:                   9443,
		LeaderElection:         cfg.LeaderElection,
		LeaderElectionID:       "784c2908.mohammadne.me",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		logger.Fatal("Unable to start manager", zap.Error(err))
	}

	if err = (&controllers.ExecuterReconciler{
		Client:     manager.GetClient(),
		Scheme:     manager.GetScheme(),
		Config:     cfg,
		Cloudflare: cloudflare.New(cfg.Cloudflare, cfg.Domain),
		Logger:     logger,
	}).SetupWithManager(manager); err != nil {
		logger.Fatal("Unable to create Executer controller", zap.Error(err))
	}
	if err = (&appsv1alpha1.Executer{}).SetupWebhookWithManager(manager); err != nil {
		logger.Fatal("Unable to create Executer webhook", zap.Error(err))
	}
	//+kubebuilder:scaffold:builder

	if err := manager.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		logger.Fatal("Unable to set up health check", zap.Error(err))
	}
	if err := manager.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		logger.Fatal("Unable to set up ready check", zap.Error(err))
	}

	logger.Info("Starting manager")
	if err := manager.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Info("Problem running manager", zap.Error(err))
	}
}
