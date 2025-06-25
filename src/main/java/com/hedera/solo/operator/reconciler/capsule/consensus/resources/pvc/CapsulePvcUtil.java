// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpecBuilder;
import io.fabric8.kubernetes.api.model.Quantity;
import java.util.Objects;

/**
 * The `CapsulePvcUtil` class provides utility methods for working with PersistentVolumeClaim (PVC) resources in the
 * context of a ConsensusCapsule.
 */
public class CapsulePvcUtil {

    /**
     * Generates a valid PVC name based on the ConsensusCapsule and volume suffix.
     *
     * @param cc The ConsensusCapsule object containing metadata about the capsule.
     * @param volSuffix The suffix to be appended to the node name in the PVC name.
     * @return A string representing the valid PVC name
     */
    public static String validPvcName(ConsensusCapsule cc, String volSuffix) {
        final String nodeName = Metadata.prepareNodeName(cc);
        return "%s-%s".formatted(nodeName, volSuffix);
    }

    /**
     * Validates the provided ConsensusCapsule object to ensure it is not null and has necessary metadata and
     * specifications.
     *
     * @param primary The ConsensusCapsule object to be validated.
     */
    public static void validate(ConsensusCapsule primary) {
        Objects.requireNonNull(primary.getMetadata());
        Objects.requireNonNull(primary.getMetadata().getNamespace());
        Objects.requireNonNull(primary.getSpec());
    }

    /**
     * Creates a desired PersistentVolumeClaim specification based on the provided input, ensuring necessary fields are
     * not null.
     *
     * @param input The initial PersistentVolumeClaim specification to be modified.
     * @param volSuffix The suffix for the volume, used in naming the PVC.
     * @return A new PersistentVolumeClaim specification with desired properties
     * @throws IllegalArgumentException if the input specification is invalid, such as having empty capacity or access
     *     modes.
     */
    public static PersistentVolumeClaimSpec desiredPvcSpec(PersistentVolumeClaimSpec input, String volSuffix) {
        Objects.requireNonNull(input);
        Objects.requireNonNull(input.getAccessModes());
        Objects.requireNonNull(input.getResources());

        if (input.getResources().getRequests() == null
                || Objects.equals(input.getResources().getRequests().get("storage"), Quantity.parse("0"))) {
            throw new IllegalArgumentException("capacity cannot be empty for PVC '%s'".formatted(volSuffix));
        }

        if (input.getAccessModes().isEmpty()) {
            throw new IllegalArgumentException("access-mode cannot be empty for PVC '%s'".formatted(volSuffix));
        }

        return new PersistentVolumeClaimSpecBuilder()
                .withAccessModes(input.getAccessModes())
                .withResources(input.getResources())
                .build();
    }

    /**
     * Prepares a desired PersistentVolumeClaim object based on the provided ConsensusCapsule, PVC specification, and
     * volume suffix.
     *
     * @param primary The ConsensusCapsule object containing metadata for naming and namespace.
     * @param pvcSpec The specification of the PersistentVolumeClaim to be created.
     * @param volSuffix The suffix used in generating the PVC name.
     * @return A new PersistentVolumeClaim object with appropriate metadata and specifications
     */
    public static PersistentVolumeClaim prepareDesired(
            ConsensusCapsule primary, PersistentVolumeClaimSpec pvcSpec, String volSuffix) {
        final PersistentVolumeClaimSpec ps = CapsulePvcUtil.desiredPvcSpec(pvcSpec, volSuffix);
        final String validName = CapsulePvcUtil.validPvcName(primary, volSuffix);
        final String namespace = primary.getMetadata().getNamespace();
        final String orbit = primary.getSpec().getOrbit();

        PersistentVolumeClaimBuilder builder = new PersistentVolumeClaimBuilder();
        return Metadata.applyStandardLabels(
                        builder.editMetadata(),
                        validName,
                        orbit,
                        Constants.COMPONENT_CONSENSUS_NODE,
                        Constants.COMPONENT_PVC,
                        null)
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata()
                .withSpec(ps)
                .build();
    }
}
