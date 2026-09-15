// SPDX-License-Identifier: Apache-2.0

package uc

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// configCRGVRs is the canonical set of config-CR GVRs the provisioner-proxy
// needs create/get/update on. Keep in lockstep with the RBAC grant in
// internal/resources/consensus/role.go.
var configCRGVRs = []schema.GroupVersionResource{
	log4j2ConfigGVR,
	nodeSettingsGVR,
	applicationPropertiesGVR,
	applicationOverridePropertiesGVR,
	bootstrapPropertiesGVR,
	nodePropertiesGVR,
	throttlesConfigGVR,
	feeSchedulesGVR,
	simpleFeesSchedulesGVR,
	apiPermissionPropertiesGVR,
	blockNodesConfigGVR,
}

// configCRVerbs are the verbs checked by the RBAC probe. UC creates config CRs
// (create) and polls them for Valid=True (get); config CRs are per-operation and
// immutable, so UC never updates them.
var configCRVerbs = []string{"create", "get"}

// RBACProbe implements daemonkit.Probe by issuing SelfSubjectAccessReview
// requests to verify the ServiceAccount can create/get every config-CR.
type RBACProbe struct {
	Client    dynamic.Interface
	Namespace string

	PollInterval time.Duration
}

const rbacProbeDefaultInterval = 5 * time.Second

var ssarGVR = schema.GroupVersionResource{
	Group:    "authorization.k8s.io",
	Version:  "v1",
	Resource: "selfsubjectaccessreviews",
}

func (p *RBACProbe) pollInterval() time.Duration {
	if p.PollInterval > 0 {
		return p.PollInterval
	}
	return rbacProbeDefaultInterval
}

// Probe blocks until the SA has all required permissions or ctx is cancelled.
func (p *RBACProbe) Probe(ctx context.Context) error {
	ticker := time.NewTicker(p.pollInterval())
	defer ticker.Stop()

	for {
		denied := p.checkAll(ctx)
		if len(denied) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("config-CR RBAC check: denied on %s: %w",
				strings.Join(denied, ", "), ctx.Err())
		case <-ticker.C:
		}
	}
}

func (p *RBACProbe) checkAll(ctx context.Context) []string {
	var denied []string
	for _, gvr := range configCRGVRs {
		for _, verb := range configCRVerbs {
			allowed, err := p.checkAccess(ctx, gvr, verb)
			if err != nil || !allowed {
				denied = append(denied, fmt.Sprintf("%s/%s", verb, gvr.Resource))
			}
		}
	}
	return denied
}

func (p *RBACProbe) checkAccess(ctx context.Context, gvr schema.GroupVersionResource, verb string) (bool, error) {
	ssar := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "authorization.k8s.io/v1",
			"kind":       "SelfSubjectAccessReview",
			"spec": map[string]interface{}{
				"resourceAttributes": map[string]interface{}{
					"namespace": p.Namespace,
					"verb":      verb,
					"group":     gvr.Group,
					"version":   gvr.Version,
					"resource":  gvr.Resource,
				},
			},
		},
	}

	result, err := p.Client.Resource(ssarGVR).Create(ctx, ssar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	allowed, _, _ := unstructured.NestedBool(result.Object, "status", "allowed")
	return allowed, nil
}

// DeniedResources returns the list of denied verb/resource pairs (for testing).
func (p *RBACProbe) DeniedResources(ctx context.Context) []string {
	return p.checkAll(ctx)
}
