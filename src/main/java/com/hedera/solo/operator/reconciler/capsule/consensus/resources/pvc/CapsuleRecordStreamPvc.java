// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;

/**
 * The CapsuleRecordStreamPvc class is a Kubernetes dependent resource that manages the creation and reconciliation of
 * PersistentVolumeClaim (PVC) resources for record stream volumes in a ConsensusCapsule.
 */
@KubernetesDependent
public class CapsuleRecordStreamPvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule> {
    /**
     * Constructs a new instance of CapsuleRecordStreamPvc with the desired Kubernetes resource type set to
     * PersistentVolumeClaim.
     */
    public CapsuleRecordStreamPvc() {
        super(PersistentVolumeClaim.class);
    }

    /**
     * Retrieves the specification for the PVC based on the input from the primary ConsensusCapsule resource.
     *
     * @param primary The primary ConsensusCapsule resource.
     * @return The PersistentVolumeClaimSpec, or null if not specified
     */
    private static PersistentVolumeClaimSpec getInputPvcSpec(final ConsensusCapsule primary) {
        if (primary.getSpec().getPersistentVolumeClaims() == null
                || primary.getSpec().getPersistentVolumeClaims().getStreams() == null) {
            return null;
        }

        return primary.getSpec().getPersistentVolumeClaims().getStreams().getRecord();
    }

    @Override
    protected PersistentVolumeClaim desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        CapsulePvcUtil.validate(primary);
        return CapsulePvcUtil.prepareDesired(primary, getInputPvcSpec(primary), VOLUME_RECORD_STREAM_NAME_SUFFIX);
    }

    /** A condition class to check if the PVC is ready for use in a ConsensusCapsule. */
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
