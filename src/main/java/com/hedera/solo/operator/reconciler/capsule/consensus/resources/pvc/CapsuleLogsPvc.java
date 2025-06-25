// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_LOGS_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;

/**
 * This class represents a Kubernetes PersistentVolumeClaim (PVC) for logs of the consensus capsule. It is configured as
 * a dependent resource of the ConsensusCapsule and manages the creation and update of the PVC based on the
 * specifications provided in the primary resource.
 */
@KubernetesDependent
public class CapsuleLogsPvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule> {
    /**
     * Default constructor for creating a new instance of CapsuleLogsPvc. Initializes the dependent resource with
     * PersistentVolumeClaim type.
     */
    public CapsuleLogsPvc() {
        super(PersistentVolumeClaim.class);
    }

    /**
     * Retrieves the desired PersistentVolumeClaim specification from the primary ConsensusCapsule resource.
     *
     * @param primary The primary ConsensusCapsule resource.
     * @return The PVC specification for logs, or null if not specified
     */
    private static PersistentVolumeClaimSpec getInputPvcSpec(final ConsensusCapsule primary) {
        if (primary.getSpec().getPersistentVolumeClaims() == null) {
            return null;
        }

        return primary.getSpec().getPersistentVolumeClaims().getLogs();
    }

    @Override
    protected PersistentVolumeClaim desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        CapsulePvcUtil.validate(primary);
        return CapsulePvcUtil.prepareDesired(primary, getInputPvcSpec(primary), VOLUME_LOGS_NAME_SUFFIX);
    }

    /** A condition class to check if the logs PVC is ready based on the primary ConsensusCapsule specification. */
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
