// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"fmt"
	"net"
	"time"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
)

// GVRs for the four NetworkUpgrade CRs the UC creates/watches. The group and
// version are taken from the operator API package so they stay in sync with the
// CRD definitions.
var (
	networkUpgradePrepareGVR = gvr("networkupgradeprepares")
	//nolint:unused // consumed by the factory CR handlers in #994
	networkUpgradeFreezeGVR  = gvr("networkupgradefreezes")
	networkUpgradeExecuteGVR = gvr("networkupgradeexecutes")
	//nolint:unused // consumed by the factory CR handlers in #996
	networkUpgradeFreezeAbortGVR = gvr("networkupgradefreezeaborts")
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
			return nil, fmt.Errorf("load in-cluster config: %w", err)
		}
	} else {
		restCfg, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("load kubeconfig %s: %w", kubeconfigPath, err)
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
		return nil, fmt.Errorf("build dynamic client: %w", err)
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
