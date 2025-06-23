// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_UPGRADE_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;

/**
 * The CapsuleUpgradePvc class is a Kubernetes dependent resource that manages the creation and updating of
 * PersistentVolumeClaim (PVC) resources for upgrade purposes in a consensus capsule.
 */
@KubernetesDependent
public class CapsuleUpgradePvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule> {
    /**
     * Constructs a new instance of CapsuleUpgradePvc with the desired Kubernetes resource type set to
     * PersistentVolumeClaim.
     */
    public CapsuleUpgradePvc() {
        super(PersistentVolumeClaim.class);
    }

    /**
     * Retrieves the specification for the persistent volume claim from the primary consensus capsule resource.
     *
     * @param primary The primary ConsensusCapsule resource.
     * @return The PersistentVolumeClaimSpec if available, otherwise null
     */
    private static PersistentVolumeClaimSpec getInputPvcSpec(final ConsensusCapsule primary) {
        if (primary.getSpec() == null || primary.getSpec().getPersistentVolumeClaims() == null) {
            return null;
        }

        return primary.getSpec().getPersistentVolumeClaims().getUpgrade();
    }

    @Override
    protected PersistentVolumeClaim desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        CapsulePvcUtil.validate(primary);
        return CapsulePvcUtil.prepareDesired(primary, getInputPvcSpec(primary), VOLUME_UPGRADE_NAME_SUFFIX);
    }

    /**
     * A condition class to check if the PersistentVolumeClaim is ready for use based on the presence of a valid
     * specification in the primary consensus capsule.
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
