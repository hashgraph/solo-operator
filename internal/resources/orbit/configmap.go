// SPDX-License-Identifier: Apache-2.0

package orbit

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/subreconciler"
)

// ConfigMapSubReconciler manages the consensus config map for an Orbit
type ConfigMapSubReconciler struct {
	*subreconciler.BaseSubReconciler[*operatorv1alpha1.Orbit]
}

// NewConfigMapSubReconciler creates a new ConfigMapSubReconciler
func NewConfigMapSubReconciler(c client.Client, scheme *runtime.Scheme) *ConfigMapSubReconciler {
	return &ConfigMapSubReconciler{
		BaseSubReconciler: subreconciler.NewBaseSubReconciler[*operatorv1alpha1.Orbit](c, scheme, "ConsensusConfigMap"),
	}
}

// Reconcile ensures the consensus config map exists
func (r *ConfigMapSubReconciler) Reconcile(ctx context.Context, orbit *operatorv1alpha1.Orbit) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	desired := r.buildConfigMap(orbit)

	// Check if exists
	existing := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Creating consensus config map", "name", desired.Name, "namespace", desired.Namespace)
			if err := r.Client.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Update data if needed
	if r.needsUpdate(existing, desired) {
		logger.V(1).Info("Updating consensus config map", "name", desired.Name, "namespace", desired.Namespace)
		existing.Data = desired.Data
		existing.Labels = desired.Labels
		if err := r.Client.Update(ctx, existing); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// buildConfigMap builds the desired config map
func (r *ConfigMapSubReconciler) buildConfigMap(orbit *operatorv1alpha1.Orbit) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConsensusConfigMapName,
			Namespace: orbit.Name,
			Labels:    StandardLabels(orbit),
		},
		Data: buildConsensusConfigData(),
	}
}

// buildConsensusConfigData builds the consensus config map data
func buildConsensusConfigData() map[string]string {
	data := make(map[string]string)

	// Empty placeholder for s6-overlay
	data["empty"] = ""

	// s6 start-consensus run script
	data["s6-start-consensus-run"] = `#!/command/execlineb -P
foreground { s6-echo "[start-consensus] starting consensus service" }
/opt/hgcapp/services-hedera/HapiApp2.0/entrypoint.sh start
`

	// s6 type for longrun service
	data["s6-type-longrun"] = "longrun"

	// s6 finish script
	data["s6-start-consensus-finish"] = `#!/command/execlineb -S0
foreground { s6-echo "[start-consensus] consensus service finished" }
s6-svscanctl -t /run/service
`

	// jshell check script for readiness probe
	data["is-consensus-active"] = `import com.hedera.node.app.service.consensus.impl.ConsensusServiceImpl;
import com.swirlds.platform.state.MerkleNodeState;
import com.swirlds.platform.state.service.PlatformStateService;
import com.swirlds.platform.system.Platform;

try {
    var platform = (Platform) com.hedera.node.app.Hedera.class.getDeclaredField("platform").get(null);
    var state = (MerkleNodeState) platform.getLatestImmutableState("readinessCheck");
    var platformState = state.getReadableStates(PlatformStateService.NAME);
    System.out.println("Platform state available: " + (platformState != null));
    System.exit(platformState != null ? 0 : 1);
} catch (Exception e) {
    System.out.println("Not ready: " + e.getMessage());
    System.exit(1);
}
/exit
`

	return data
}

// needsUpdate checks if the config map needs to be updated
func (r *ConfigMapSubReconciler) needsUpdate(existing, desired *corev1.ConfigMap) bool {
	// Check data equality
	if len(existing.Data) != len(desired.Data) {
		return true
	}
	for k, v := range desired.Data {
		if existing.Data[k] != v {
			return true
		}
	}
	// Check labels
	for k, v := range desired.Labels {
		if existing.Labels[k] != v {
			return true
		}
	}
	return false
}
