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

public class CapsulePvcUtil {
    public static String validPvcName(ConsensusCapsule cc, String volSuffix) {
        final String nodeId = Metadata.prepareNodeName(cc);
        return "%s-%s".formatted(nodeId, volSuffix);
    }

    public static void validate(ConsensusCapsule primary) {
        Objects.requireNonNull(primary.getMetadata());
        Objects.requireNonNull(primary.getMetadata().getNamespace());
        Objects.requireNonNull(primary.getSpec());
    }

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
