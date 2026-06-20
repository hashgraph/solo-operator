// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"fmt"
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
	//nolint:unused // consumed by the factory CR handlers in #993
	networkUpgradePrepareGVR = gvr("networkupgradeprepares")
	//nolint:unused // consumed by the factory CR handlers in #994
	networkUpgradeFreezeGVR  = gvr("networkupgradefreezes")
	networkUpgradeExecuteGVR = gvr("networkupgradeexecutes")
	//nolint:unused // consumed by the factory CR handlers in #996
	networkUpgradeFreezeAbortGVR = gvr("networkupgradefreezeaborts")
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
	restCfg.Timeout = 30 * time.Second
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
