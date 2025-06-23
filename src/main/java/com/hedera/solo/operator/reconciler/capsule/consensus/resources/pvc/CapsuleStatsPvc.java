// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_STATS_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;

/**
 * The `CapsuleStatsPvc` class is responsible for managing the desired state of a PersistentVolumeClaim (PVC) for
 * statistics data in a Kubernetes cluster.
 *
 * <p>The primary purpose of this class is to ensure that the desired state of the PVC matches the specifications
 * defined in the `ConsensusCapsule` resource. It also provides a condition to check if the PVC is ready.
 */
@KubernetesDependent
public class CapsuleStatsPvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule> {
    /** Constructs a new instance of `CapsuleStatsPvc`, initializing it to manage PersistentVolumeClaim resources. */
    public CapsuleStatsPvc() {
        super(PersistentVolumeClaim.class);
    }

    /**
     * Retrieves the specification for the persistent volume claim from the primary resource's spec, if available.
     *
     * @param primary The `ConsensusCapsule` resource which serves as the primary resource for this dependent resource.
     * @return The specification of the PVC, or null if not specified
     */
    private static PersistentVolumeClaimSpec getInputPvcSpec(final ConsensusCapsule primary) {
        if (primary.getSpec().getPersistentVolumeClaims() == null) {
            return null;
        }

        return primary.getSpec().getPersistentVolumeClaims().getStats();
    }

    @Override
    protected PersistentVolumeClaim desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        CapsulePvcUtil.validate(primary);
        return CapsulePvcUtil.prepareDesired(primary, getInputPvcSpec(primary), VOLUME_STATS_NAME_SUFFIX);
    }

    /**
     * Defines a condition to check if the PVC is ready based on whether it has been specified in the `ConsensusCapsule`
     * resource.
     */
    public static class Ready implements Condition<PersistentVolumeClaim, ConsensusCapsule> {
        @Override
        public boolean isMet(
                DependentResource<PersistentVolumeClaim, ConsensusCapsule> dependentResource,
                ConsensusCapsule primary,
                Context<ConsensusCapsule> context) {
            return getInputPvcSpec(primary) != null;
        }
    }
}
