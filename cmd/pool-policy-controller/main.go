package main

import (
	"context"
	"flag"
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"

	poolpolicy "github.com/llm-d-incubation/llm-d-fast-model-actuation/pkg/controller/pool-policy"
)

func main() {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}

	klog.InitFlags(flag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	AddFlags(*pflag.CommandLine, loadingRules, overrides)
	pflag.Parse()

	// Optional: a namespace can be provided to limit the manager's watch scope.

	// Build kubeconfig from the environment / kubeconfig flags
	restCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides).ClientConfig()
	if err != nil {
		klog.Fatal(err)
	}
	if len(restCfg.UserAgent) == 0 {
		restCfg.UserAgent = "pool-policy-controller"
	} else {
		restCfg.UserAgent += "/pool-policy-controller"
	}

	ctx := context.Background()
	logger := klog.FromContext(ctx)

	pflag.CommandLine.VisitAll(func(f *pflag.Flag) {
		logger.V(1).Info("Flag", "name", f.Name, "value", f.Value.String())
	})

	// Create manager (controller-runtime v0.22 does not support restricting
	// namespaces via Options.Namespace in this version). The optional
	// --namespace flag is informational for this binary.
	mgr, err := ctrl.NewManager(restCfg, ctrl.Options{})
	if err != nil {
		klog.Fatal(err)
	}
	ppReconciler := &poolpolicy.Reconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Logger: logger}
	if err := ppReconciler.SetupWithManager(mgr); err != nil {
		klog.Fatal(err)
	}

	if overrides.Context.Namespace == "" {
		klog.Info("starting poolpolicy controller", "watchNamespace", "<all>")
	} else {
		klog.Info("starting poolpolicy controller", "watchNamespace", overrides.Context.Namespace)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		klog.Fatalf("manager exited: %v", err)
	}

	// small sleep to ensure logs flush
	time.Sleep(100 * time.Millisecond)
}

func AddFlags(flags pflag.FlagSet, loadingRules *clientcmd.ClientConfigLoadingRules, overrides *clientcmd.ConfigOverrides) {
	flags.StringVar(&loadingRules.ExplicitPath, "kubeconfig", loadingRules.ExplicitPath, "Path to the kubeconfig file to use")
	flags.StringVar(&overrides.CurrentContext, "context", overrides.CurrentContext, "The name of the kubeconfig context to use")
	flags.StringVar(&overrides.Context.AuthInfo, "user", overrides.Context.AuthInfo, "The name of the kubeconfig user to use")
	flags.StringVar(&overrides.Context.Cluster, "cluster", overrides.Context.Cluster, "The name of the kubeconfig cluster to use")
	flags.StringVarP(&overrides.Context.Namespace, "namespace", "n", overrides.Context.Namespace, "The name of the Kubernetes Namespace to work in (NOT optional)")
}
