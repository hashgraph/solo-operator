// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"net"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/errx"
)

// GVRs for the four NetworkUpgrade CRs the UC creates/watches. The group and
// version are taken from the operator API package so they stay in sync with the
// CRD definitions.
var (
	networkUpgradePrepareGVR     = gvr("networkupgradeprepares")
	networkUpgradeFreezeGVR      = gvr("networkupgradefreezes")
	networkUpgradeExecuteGVR     = gvr("networkupgradeexecutes")
	networkUpgradeFreezeAbortGVR = gvr("networkupgradefreezeaborts")

	// consensusCapsuleGVR is the capsule the platform-status sensor patches
	// (status.platformStatus / status.platformStatusCode).
	consensusCapsuleGVR = gvr("consensuscapsules")

	// GVRs for the ten ConsensusConfig CRs the provisioner-proxy creates during
	// the execute phase (cluster-only mode). The resource names are the lowercase
	// plural of each Kind, matching the CRD spec (HIP-1496 §§ filename-to-CR mapping).
	log4j2ConfigGVR                  = gvr("log4j2configs")
	nodeSettingsGVR                  = gvr("nodesettings")
	applicationPropertiesGVR         = gvr("applicationproperties")
	applicationOverridePropertiesGVR = gvr("applicationoverrideproperties")
	bootstrapPropertiesGVR           = gvr("bootstrapproperties")
	nodePropertiesGVR                = gvr("nodepropertiesconfigs")
	throttlesConfigGVR               = gvr("throttlesconfigs")
	feeSchedulesGVR                  = gvr("feeschedules")
	simpleFeesSchedulesGVR           = gvr("simplefeesschedules")
	apiPermissionPropertiesGVR       = gvr("apipermissionproperties")

	// stepToGVR maps a CR step label value to the GVR of the corresponding
	// NetworkUpgrade* resource. Used by restart recovery (#998) to re-deliver
	// persisted CRs and to query Execute CRs by step without hard-coding strings
	// at multiple call sites.
	stepToGVR = map[string]schema.GroupVersionResource{
		operatorv1alpha1.StepPrepare: networkUpgradePrepareGVR,
		operatorv1alpha1.StepFreeze:  networkUpgradeFreezeGVR,
		operatorv1alpha1.StepExecute: networkUpgradeExecuteGVR,
		operatorv1alpha1.StepAbort:   networkUpgradeFreezeAbortGVR,
	}

	// stepToMarker maps a CR step label value to the marker file that triggered it.
	// Used to include the file name in ERROR logs when a CR is in Failed state so
	// the operator knows which marker file to delete to clear the stuck upgrade.
	stepToMarker = map[string]string{
		operatorv1alpha1.StepPrepare: MarkerExecuteImmediate,
		operatorv1alpha1.StepFreeze:  MarkerFreezeScheduled,
		operatorv1alpha1.StepExecute: MarkerNowFrozen,
		operatorv1alpha1.StepAbort:   MarkerFreezeAborted,
	}

	// allUpgradeGVRs is the set the startup op-id cache seed (#997) and restart
	// recovery (#998) iterate over to discover CRs already in the cluster.
	allUpgradeGVRs = []schema.GroupVersionResource{
		networkUpgradePrepareGVR,
		networkUpgradeFreezeGVR,
		networkUpgradeExecuteGVR,
		networkUpgradeFreezeAbortGVR,
	}
)

const (
	// dialTimeout bounds TCP connection establishment (the SYN-no-reply case) and
	// is applied via rest.Config.Dial. Unlike rest.Config.Timeout it does NOT cap
	// an established stream, so it protects against connect hangs without killing
	// a long-lived watch mid-flight.
	dialTimeout = 30 * time.Second

	// listTimeout bounds a single List call. It is applied per-call via
	// context.WithTimeout and must NOT be applied to the watch (its lifetime is
	// governed by watch TimeoutSeconds server-side); see buildDynamicClient for
	// why a client-wide rest.Config.Timeout is unsafe for watches.
	listTimeout = 30 * time.Second

	// createTimeout bounds a single CR Create call. Applied per-call via
	// context.WithTimeout so a stalled API server cannot block the marker poll
	// loop indefinitely — the handler fails fast and retries on the next tick.
	createTimeout = 30 * time.Second

	// configCRDeployTimeout is the wall-clock budget for the entire config-CR
	// deployment step inside handleExecute (cluster-only). It covers scanning,
	// creating all config CRs, and waiting for the operator to reconcile each
	// one to Valid=True. 20 minutes is generous; the operator reconciles
	// ConfigMap-backed CRs within seconds under normal conditions.
	configCRDeployTimeout = 20 * time.Minute

	// configCRPollInterval is the fallback poll cadence used when
	// ProvisionerProxyConfig.ConfigCRPollInterval is zero (unset). Prefer
	// setting ConfigCRPollInterval via --config-cr-poll-interval / UC_CONFIG_CR_POLL_INTERVAL.
	configCRPollInterval = 1 * time.Second
)

func gvr(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    operatorv1alpha1.GroupVersion.Group,
		Version:  operatorv1alpha1.GroupVersion.Version,
		Resource: resource,
	}
}

// buildDynamicClient builds a dynamic Kubernetes client. An empty kubeconfigPath
// selects in-cluster configuration (the normal sidecar case). restCfg.Timeout is
// bounded so a TCP-level hang cannot block Watch() for the OS TCP timeout.
func buildDynamicClient(kubeconfigPath string) (dynamic.Interface, error) {
	var (
		restCfg *rest.Config
		err     error
	)
	if kubeconfigPath == "" {
		restCfg, err = rest.InClusterConfig()
		if err != nil {
			return nil, errx.WithHints(
				errx.WithReason(errClient.Wrap(err, "load in-cluster config"), ReasonInClusterConfigFailed),
				"Run the UC as a pod with a mounted ServiceAccount token, or pass --kubeconfig for out-of-cluster use",
			)
		}
	} else {
		restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, errx.WithHints(
				errx.WithReason(errClient.Wrap(err, "load kubeconfig %s", kubeconfigPath), ReasonKubeconfigLoadFailed),
				"Confirm --kubeconfig points to a readable, valid kubeconfig file",
			)
		}
	}
	// Bound TCP connection establishment via a dial timeout — NOT rest.Config.Timeout,
	// which becomes http.Client.Timeout and caps the whole request (including the
	// watch response-body read), terminating Watch() early regardless of
	// ListOptions.TimeoutSeconds (server-side only). The dial timeout bounds the
	// SYN-no-reply connect hang; KeepAlive detects a silently-dead established
	// connection — neither limits the lifetime of a healthy watch stream.
	restCfg.Dial = (&net.Dialer{Timeout: dialTimeout, KeepAlive: dialTimeout}).DialContext
	client, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return nil, errx.WithReason(errClient.Wrap(err, "build dynamic client"), ReasonDynamicClientFailed)
	}
	return client, nil
}

// isAuthError returns true for HTTP 401/403 responses from the K8s API.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	return k8serrors.IsUnauthorized(err) || k8serrors.IsForbidden(err)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
