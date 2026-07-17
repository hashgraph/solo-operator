// SPDX-License-Identifier: Apache-2.0

package consensus

import (
	"context"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

type RoleSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.ConsensusCapsule]
}

func NewRoleSubReconciler(c client.Client, scheme *runtime.Scheme) *RoleSubReconciler {
	return &RoleSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.ConsensusCapsule](c, scheme, "Role"),
	}
}

func (r *RoleSubReconciler) Reconcile(ctx context.Context, capsule *operatorv1alpha1.ConsensusCapsule) (ctrl.Result, error) {
	desired := r.buildRole(capsule)
	if err := controllerutil.SetControllerReference(capsule, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	existing := &rbacv1.Role{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if apierrors.IsNotFound(err) {
		common.Info(ctx, common.ReasonRoleCreated).Str("role", desired.Name).Msg("Creating role")
		return ctrl.Result{}, r.Client.Create(ctx, desired)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	if !rulesEqual(existing.Rules, desired.Rules) || !labelsMatch(existing.Labels, desired.Labels) {
		existing.Rules = desired.Rules
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *RoleSubReconciler) buildRole(capsule *operatorv1alpha1.ConsensusCapsule) *rbacv1.Role {
	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get", "list", "watch"}},
		{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list", "watch"}},
	}

	uc := capsule.Spec.PodProperties.Containers.UC
	if uc != nil && uc.Enabled {
		// The UC sidecar creates NetworkUpgrade* CRs and watches them by spec.operationId via
		// the field indexer. The provisioner-proxy also patches NetworkUpgradeExecute status.
		networkUpgradeResources := []string{
			"networkupgradeprepares",
			"networkupgradefreezes",
			"networkupgradeexecutes",
			"networkupgradefreezeaborts",
		}
		rules = append(rules,
			rbacv1.PolicyRule{
				APIGroups: []string{"operator.solo.hiero.org"},
				Resources: networkUpgradeResources,
				Verbs:     []string{"create", "get", "list", "watch"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"operator.solo.hiero.org"},
				Resources: []string{"networkupgradeexecutes/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			// The platform-status sensor patches its own ConsensusCapsule
			// status.platformStatus (sole writer, server-side merge-patch only).
			rbacv1.PolicyRule{
				APIGroups: []string{"operator.solo.hiero.org"},
				Resources: []string{"consensuscapsules/status"},
				Verbs:     []string{"patch"},
			},
		)
	}

	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RoleName(capsule),
			Namespace: capsule.Namespace,
			Labels:    CommonLabels(capsule),
		},
		Rules: rules,
	}
}

func rulesEqual(a, b []rbacv1.PolicyRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !stringSliceEq(a[i].APIGroups, b[i].APIGroups) ||
			!stringSliceEq(a[i].Resources, b[i].Resources) ||
			!stringSliceEq(a[i].Verbs, b[i].Verbs) {
			return false
		}
	}
	return true
}

func stringSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func labelsMatch(existing, desired map[string]string) bool {
	for k, v := range desired {
		if existing[k] != v {
			return false
		}
	}
	return true
}
