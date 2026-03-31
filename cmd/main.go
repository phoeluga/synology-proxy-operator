// main is the entry point for the Synology Proxy Operator.
// It sets up the controller-runtime manager and registers both reconcilers.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	// Import all Kubernetes client auth plugins.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	proxyv1alpha1 "github.com/phoeluga/synology-proxy-operator/api/v1alpha1"
	"github.com/phoeluga/synology-proxy-operator/internal/argo"
	"github.com/phoeluga/synology-proxy-operator/internal/controller"
	"github.com/phoeluga/synology-proxy-operator/internal/synology"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(networkingv1.AddToScheme(scheme))
	utilruntime.Must(proxyv1alpha1.AddToScheme(scheme))
	utilruntime.Must(argo.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string

		// Synology DSM connection.
		synologyURL           string
		synologyUser          string
		synologyPassword      string
		synologySkipTLSVerify bool

		// Operator behaviour.
		defaultACLProfile string
		defaultDomain     string
		watchNamespace    string
		ruleNamespace     string
		enableArgoWatcher bool
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")

	flag.StringVar(&synologyURL, "synology-url", envOrDefault("SYNOLOGY_URL", ""), "Synology DSM base URL (e.g. https://diskstation.local:5001).")
	flag.StringVar(&synologyUser, "synology-user", envOrDefault("SYNOLOGY_USER", ""), "Synology DSM username.")
	flag.StringVar(&synologyPassword, "synology-password", envOrDefault("SYNOLOGY_PASSWORD", ""), "Synology DSM password.")
	flag.BoolVar(&synologySkipTLSVerify, "synology-skip-tls-verify", envBoolOrDefault("SYNOLOGY_SKIP_TLS_VERIFY", false), "Skip TLS verification for Synology DSM.")

	flag.StringVar(&defaultACLProfile, "default-acl-profile", envOrDefault("DEFAULT_ACL_PROFILE", ""), "Default ACL profile name applied to all proxy rules.")
	flag.StringVar(&defaultDomain, "default-domain", envOrDefault("DEFAULT_DOMAIN", ""), "Default domain suffix for auto-generated source hostnames (e.g. example.com).")
	flag.StringVar(&watchNamespace, "watch-namespace", envOrDefault("WATCH_NAMESPACE", ""), "Namespace or glob pattern (e.g. app-*) for auto-enabling proxy rules without annotations. Applies to Services, Ingresses and ArgoCD Applications. Empty = annotation-only mode.")
	flag.StringVar(&ruleNamespace, "rule-namespace", envOrDefault("RULE_NAMESPACE", "synology-proxy-operator"), "Namespace where SynologyProxyRule objects are created.")
	flag.BoolVar(&enableArgoWatcher, "enable-argo-watcher", envBoolOrDefault("ENABLE_ARGO_WATCHER", true), "Enable the ArgoCD Application watcher.")

	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// Validate required flags.
	if synologyURL == "" || synologyUser == "" || synologyPassword == "" {
		setupLog.Error(fmt.Errorf("missing required configuration"),
			"synology-url, synology-user, and synology-password must all be set")
		os.Exit(1)
	}

	// Build Synology client.
	synClient, err := synology.New(synology.Config{
		URL:           synologyURL,
		Username:      synologyUser,
		Password:      synologyPassword,
		SkipTLSVerify: synologySkipTLSVerify,
	}, ctrl.Log.WithName("synology-client"))
	if err != nil {
		setupLog.Error(err, "Failed to create Synology client")
		os.Exit(1)
	}

	// Build manager options.
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "proxy.synology.io",
	})
	if err != nil {
		setupLog.Error(err, "Unable to create manager")
		os.Exit(1)
	}

	// Register SynologyProxyRule reconciler.
	if err := (&controller.SynologyProxyRuleReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Log:               ctrl.Log.WithName("controllers").WithName("SynologyProxyRule"),
		SynologyClient:    synClient,
		DefaultACLProfile: defaultACLProfile,
		DefaultDomain:     defaultDomain,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create SynologyProxyRule controller")
		os.Exit(1)
	}

	// Register Service/Ingress watcher.
	if err := (&controller.ServiceIngressReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Log:            ctrl.Log.WithName("controllers").WithName("ServiceIngress"),
		RuleNamespace:  ruleNamespace,
		WatchNamespace: watchNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "Unable to create ServiceIngress controller")
		os.Exit(1)
	}

	// Register ArgoCD Application watcher (optional).
	if enableArgoWatcher {
		if err := (&controller.ArgoApplicationReconciler{
			Client:         mgr.GetClient(),
			Scheme:         mgr.GetScheme(),
			Log:            ctrl.Log.WithName("controllers").WithName("ArgoApplication"),
			DefaultDomain:  defaultDomain,
			WatchNamespace: watchNamespace,
			RuleNamespace:  ruleNamespace,
		}).SetupWithManager(mgr); err != nil {
			if strings.Contains(err.Error(), "no kind is registered") {
				setupLog.Info("ArgoCD CRDs not found in cluster — ArgoCD watcher disabled. " +
					"Install ArgoCD or set --enable-argo-watcher=false to suppress this message.")
			} else {
				setupLog.Error(err, "Unable to create ArgoApplication controller")
				os.Exit(1)
			}
		}
	}

	// Health/readiness probes.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "Unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("Starting manager",
		"synologyURL", synologyURL,
		"defaultDomain", defaultDomain,
		"ruleNamespace", ruleNamespace,
		"watchNamespace", watchNamespace,
		"argoWatcher", enableArgoWatcher,
	)

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "Problem running manager")
		os.Exit(1)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBoolOrDefault(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
