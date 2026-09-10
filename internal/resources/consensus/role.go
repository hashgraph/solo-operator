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
	"github.com/hashgraph/solo-operator/internal/uc"
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
	// The config-CR grant is needed only in cluster-only mode, where the UC's
	// provisioner-proxy stages the ConsensusConfig CRs itself. In mainnet mode
	// (host provisioner daemon) the UC never creates them, so we withhold the
	// grant to keep the node ServiceAccount least-privilege. Resolve the mode
	// only when the sidecar is enabled to avoid an unnecessary Orbit lookup.
	clusterOnly := false
	if sidecar := capsule.Spec.PodProperties.Containers.UC; sidecar != nil && sidecar.Enabled {
		mode, err := ResolveUCMode(ctx, r.Client, capsule)
		if err != nil {
			return ctrl.Result{}, err
		}
		clusterOnly = mode == uc.ModeClusterOnly
	}

	desired := r.buildRole(capsule, clusterOnly)
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
		return ctrl.Result{}, common.WarnOnConflict(ctx, r.Client.Update(ctx, existing))
	}

	return ctrl.Result{}, nil
}

func (r *RoleSubReconciler) buildRole(capsule *operatorv1alpha1.ConsensusCapsule, clusterOnly bool) *rbacv1.Role {
	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get", "list", "watch"}},
		{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list", "watch"}},
	}

	sidecar := capsule.Spec.PodProperties.Containers.UC
	if sidecar != nil && sidecar.Enabled {
		// The UC sidecar creates NetworkUpgrade* CRs and watches them by spec.operationId via
		// the field indexer. The provisioner-proxy also patches NetworkUpgradeExecute status.
		// These are needed in both mainnet and cluster-only mode.
		networkUpgradeResources := []string{
			"networkupgradeprepares",
			"networkupgradefreezes",
			"networkupgradeexecutes",
			"networkupgradefreezeaborts",
		}
		// patch is needed so the UC can stamp lifecycle=archived on terminal CRs
		// after completing its side effects (package rotation, discard, audit).
		rules = append(rules,
			rbacv1.PolicyRule{
				APIGroups: []string{"operator.solo.hiero.org"},
				Resources: networkUpgradeResources,
				Verbs:     []string{"create", "get", "list", "patch", "watch"},
			},
			rbacv1.PolicyRule{
				APIGroups: []string{"operator.solo.hiero.org"},
				Resources: []string{"networkupgradeexecutes/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
			// The platform-status sensor patches the operator.solo.hiero.org/platform-status
			// condition onto its OWN pod (pods/status, strategic merge). It never touches the
			// ConsensusCapsule status — the operator is the sole writer of that. resourceNames
			// scopes the grant to the capsule's single StatefulSet pod ("<capsule>-0"), so the
			// UC cannot patch any other pod's status in the namespace (patch supports
			// resourceNames; the pod name is deterministic for a one-replica StatefulSet).
			rbacv1.PolicyRule{
				APIGroups:     []string{""},
				Resources:     []string{"pods/status"},
				Verbs:         []string{"patch"},
				ResourceNames: []string{PodName(capsule)},
			},
		)

		// Config-CR grant — cluster-only mode ONLY. In this mode the UC's
		// provisioner-proxy creates the ConsensusConfig CR family from the deployment
		// package's data/config/ files (internal/uc/config_cr.go createConfigCR: Create,
		// with AlreadyExists treated as success), then polls each for Valid=True (Get).
		// The CRs are per-operation and immutable, so the UC never updates them. In
		// mainnet mode the host provisioner daemon owns config delivery and the UC never
		// creates these, so the grant is withheld to keep the ServiceAccount
		// least-privilege.
		//
		// Resource plurals must match the CRD spec.names.plural (and internal/uc/k8s.go
		// GVRs) verbatim, so an RBAC allow actually matches the API server's registered
		// resource. Kinds already ending in "s" keep that as their plural
		// (ApplicationProperties -> applicationproperties); the rest take the usual
		// trailing "s" (Log4j2Config -> log4j2configs).
		if clusterOnly {
			configResources := []string{
				"log4j2configs",
				"nodesettings",
				"applicationproperties",
				"applicationoverrideproperties",
				"bootstrapproperties",
				"nodepropertiesconfigs",
				"throttlesconfigs",
				"feeschedules",
				"simplefeesschedules",
				"apipermissionproperties",
				"blocknodesconfigs",
			}
			rules = append(rules, rbacv1.PolicyRule{
				APIGroups: []string{"operator.solo.hiero.org"},
				Resources: configResources,
				Verbs:     []string{"create", "get", "list", "watch", "patch"},
			})
		}
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
			!stringSliceEq(a[i].Verbs, b[i].Verbs) ||
			!stringSliceEq(a[i].ResourceNames, b[i].ResourceNames) {
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
