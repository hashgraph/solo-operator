// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_STATES_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;

/**
 * The {@code CapsuleStatesPvc} class represents a Kubernetes dependent resource for managing the persistent volume
 * claim (PVC) of consensus capsule states.
 */
@KubernetesDependent
public class CapsuleStatesPvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule> {
    /** Constructs a new {@code CapsuleStatesPvc} instance. */
    public CapsuleStatesPvc() {
        super(PersistentVolumeClaim.class);
    }

    /**
     * Retrieves the desired PVC specification from the primary consensus capsule resource.
     *
     * @param primary The primary ConsensusCapsule resource.
     * @return The PVC specification or null if not present
     */
    private static PersistentVolumeClaimSpec getInputPvcSpec(final ConsensusCapsule primary) {
        if (primary.getSpec().getPersistentVolumeClaims() == null) {
            return null;
        }

        return primary.getSpec().getPersistentVolumeClaims().getStates();
    }

    @Override
    protected PersistentVolumeClaim desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        CapsulePvcUtil.validate(primary);
        return CapsulePvcUtil.prepareDesired(primary, getInputPvcSpec(primary), VOLUME_STATES_NAME_SUFFIX);
    }

    /** Condition class to check if the PVC for capsule states is ready. */
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
