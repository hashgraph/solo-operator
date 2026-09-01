// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/automa-saga/logx"
	"github.com/automa-saga/version"
	"github.com/go-logr/logr"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/controller"
	"github.com/hashgraph/solo-operator/internal/registry"
	"github.com/hashgraph/solo-operator/internal/resources/helm"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var registryOrder string
	var allowedRegistryHosts string
	var nodeActiveTimeout time.Duration
	var nodeActiveAlertInterval time.Duration
	var showVersion bool
	var logLevel string
	var consoleLogging bool
	var tlsOpts []func(*tls.Config)

	flag.BoolVar(&showVersion, "version", false, "Print version information and exit.")
	flag.StringVar(&logLevel, "log-level", "info",
		"Log level: trace, debug, info, warn, error.")
	flag.BoolVar(&consoleLogging, "console-logging", false,
		"If set, logs use the human-readable console writer instead of structured JSON.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics server")
	flag.StringVar(&registryOrder, "registry-order", registry.StrategyRandom,
		"Order in which registries are checked during the upgrade Prepare phase: Random or Sequential.")
	flag.StringVar(&allowedRegistryHosts, "allowed-registry-hosts", "",
		"Comma-separated allow-list of registry hosts (or parent domains) permitted for image checks. "+
			"Empty (default) permits any host except loopback/link-local/private-network addresses.")
	flag.DurationVar(&nodeActiveTimeout, "node-active-timeout", 10*time.Minute,
		"Backstop deadline for a consensus node to reach platform status ACTIVE after an upgrade "+
			"(anchored to nodeUpgradeStartTime) before a NodeActiveDeadlineExceeded warning is emitted. "+
			"Increase for large states where event replay/reconnect is slow.")
	flag.DurationVar(&nodeActiveAlertInterval, "node-active-alert-interval", 5*time.Minute,
		"Interval between repeated NodeActiveDeadlineExceeded warning events after the initial "+
			"deadline fires. Shorter values give the node operator more frequent visibility.")

	flag.Parse()

	var allowedRegistryHostsList []string
	for _, h := range strings.Split(allowedRegistryHosts, ",") {
		if h = strings.TrimSpace(h); h != "" {
			allowedRegistryHostsList = append(allowedRegistryHostsList, h)
		}
	}

	if showVersion {
		fmt.Println(version.Get().Text())
		os.Exit(0)
	}

	if err := logx.Initialize(logx.LoggingConfig{
		Level:          logLevel,
		ConsoleLogging: consoleLogging,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logging: %v\n", err)
		os.Exit(1)
	}
	// Stamp the build identity onto every log line so operators can tell which
	// build is running. Image tags are reused (CI stamps 0.0.0-<sha>) and the
	// version may be a fixed placeholder, so build_commit is the always-reliable
	// discriminator. Done before SetLogger so controller-runtime's logr lines
	// inherit the fields too.
	logx.SetLogger(logx.As().With().
		Str("build_version", version.Version).
		Str("build_commit", version.Commit).
		Logger())
	// Route controller-runtime's own logs through logx.
	ctrl.SetLogger(logr.FromSlogHandler(logx.NewSlogHandler()))
	bi := version.Get()
	setupLog.Info("starting solo-operator manager",
		"build_date", bi.Date, "go_version", bi.GoVersion)

	// If the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being affected by the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs.
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'.
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		// LeaderElectionID matches the API group (operator.solo.hiero.org) so the lock
		// name is consistent with CRD group, finalizer, and label key naming conventions.
		LeaderElectionID: "operator.solo.hiero.org",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup controllers
	if err = controller.NewOrbitReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Orbit")
		os.Exit(1)
	}

	consensusCapsuleReconciler, err := controller.NewConsensusCapsuleReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		mgr.GetConfig(),
	)
	if err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ConsensusCapsule")
		os.Exit(1)
	}
	if err = consensusCapsuleReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ConsensusCapsule")
		os.Exit(1)
	}

	if err = controller.NewNetworkGenesisReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkGenesis")
		os.Exit(1)
	}

	if err = controller.NewHAProxyCapsuleReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HAProxyCapsule")
		os.Exit(1)
	}

	if err = controller.NewEnvoyProxyReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "EnvoyProxy")
		os.Exit(1)
	}

	if err = controller.NewHelmCapsuleReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
		helm.NewHelmSDKClient(mgr.GetConfig()),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "HelmCapsule")
		os.Exit(1)
	}

	if err = controller.NewLog4j2ConfigReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Log4j2Config")
		os.Exit(1)
	}

	if err = controller.NewNodeSettingsReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeSettings")
		os.Exit(1)
	}

	if err = controller.NewApplicationPropertiesReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ApplicationProperties")
		os.Exit(1)
	}

	if err = controller.NewApiPermissionPropertiesReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ApiPermissionProperties")
		os.Exit(1)
	}

	if err = controller.NewNodePropertiesReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NodeProperties")
		os.Exit(1)
	}

	if err = controller.NewThrottlesConfigReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ThrottlesConfig")
		os.Exit(1)
	}

	if err = controller.NewBlockNodesConfigReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BlockNodesConfig")
		os.Exit(1)
	}

	if err = controller.NewFeeSchedulesReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "FeeSchedules")
		os.Exit(1)
	}

	if err = controller.NewSimpleFeesSchedulesReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "SimpleFeesSchedules")
		os.Exit(1)
	}

	if err = controller.NewApplicationOverridePropertiesReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ApplicationOverrideProperties")
		os.Exit(1)
	}

	if err = controller.NewBootstrapPropertiesReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "BootstrapProperties")
		os.Exit(1)
	}

	if err = controller.RegisterCapsuleOrbitIndex(mgr); err != nil {
		setupLog.Error(err, "unable to register capsule orbit index")
		os.Exit(1)
	}
	if err = controller.RegisterPrepareIndex(mgr); err != nil {
		setupLog.Error(err, "unable to register operationId index")
		os.Exit(1)
	}
	if err = controller.RegisterFreezeIndex(mgr); err != nil {
		setupLog.Error(err, "unable to register freeze operationId index")
		os.Exit(1)
	}
	if err = controller.RegisterExecuteIndex(mgr); err != nil {
		setupLog.Error(err, "unable to register execute operationId index")
		os.Exit(1)
	}
	if err = controller.RegisterFreezeAbortIndex(mgr); err != nil {
		setupLog.Error(err, "unable to register freeze-abort operationId index")
		os.Exit(1)
	}

	prepareReconciler := controller.NewNetworkUpgradePrepareReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).WithStrategy(registryOrder).WithAllowedRegistryHosts(allowedRegistryHostsList)
	if err = prepareReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkUpgradePrepare")
		os.Exit(1)
	}

	freezeReconciler := controller.NewNetworkUpgradeFreezeReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).WithStrategy(registryOrder).WithAllowedRegistryHosts(allowedRegistryHostsList)
	if err = freezeReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkUpgradeFreeze")
		os.Exit(1)
	}

	executeReconciler := controller.NewNetworkUpgradeExecuteReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).WithStrategy(registryOrder).WithNodeActiveTimeout(nodeActiveTimeout).WithNodeActiveAlertInterval(nodeActiveAlertInterval).
		WithAllowedRegistryHosts(allowedRegistryHostsList)
	if err = executeReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkUpgradeExecute")
		os.Exit(1)
	}

	if err = controller.NewNetworkUpgradeFreezeAbortReconciler(
		mgr.GetClient(),
		mgr.GetScheme(),
	).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkUpgradeFreezeAbort")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	info := version.Get()
	setupLog.Info("starting manager", "version", info.Version, "commit", info.Commit, "buildDate", info.Date)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
