// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;

/**
 * The {@code CapsuleEventsStreamPvc} class represents a Kubernetes PersistentVolumeClaim (PVC) for events stream within
 * a ConsensusCapsule.
 */
@KubernetesDependent
public class CapsuleEventsStreamPvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule> {

    /**
     * Constructs a new instance of {@code CapsuleEventsStreamPvc}. It initializes the superclass with the type
     * {@link PersistentVolumeClaim}.
     */
    public CapsuleEventsStreamPvc() {
        super(PersistentVolumeClaim.class);
    }

    /**
     * Retrieves the desired specification for a PersistentVolumeClaim based on the input ConsensusCapsule. This method
     * checks if the spec is present in the primary resource and returns it accordingly.
     *
     * @param primary The {@link ConsensusCapsule} instance representing the primary Kubernetes resource.
     * @return The {@link PersistentVolumeClaimSpec} for events stream, or null if not specified
     */
    private static PersistentVolumeClaimSpec getInputPvcSpec(final ConsensusCapsule primary) {
        if (primary.getSpec().getPersistentVolumeClaims() == null
                || primary.getSpec().getPersistentVolumeClaims().getStreams() == null) {
            return null;
        }

        return primary.getSpec().getPersistentVolumeClaims().getStreams().getEvents();
    }

    @Override
    protected PersistentVolumeClaim desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        CapsulePvcUtil.validate(primary);
        return CapsulePvcUtil.prepareDesired(primary, getInputPvcSpec(primary), VOLUME_EVENTS_STREAM_NAME_SUFFIX);
    }

    /**
     * The {@code Ready} class implements the {@link Condition} interface to check if the PVC for events stream is
     * ready.
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
