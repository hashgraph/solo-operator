// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpecBuilder;
import io.fabric8.kubernetes.api.model.Quantity;
import java.util.Objects;

class CapsulePvcUtil {
    public static String validPvcName(ConsensusCapsule cc, String volSuffix) {
        final String nodeId = Metadata.prepareNodeName(cc);
        return "%s-%s".formatted(nodeId, volSuffix);
    }

    public static void validate(ConsensusCapsule primary) {
        Objects.requireNonNull(primary.getMetadata());
        Objects.requireNonNull(primary.getMetadata().getNamespace());
        Objects.requireNonNull(primary.getSpec());
    }

    public static PersistentVolumeClaim prepareDesired(
            ConsensusCapsule primary, PersistentVolumeClaim input, String volSuffix) {

        Objects.requireNonNull(input);
        Objects.requireNonNull(input.getSpec());
        Objects.requireNonNull(input.getSpec().getAccessModes());
        Objects.requireNonNull(input.getSpec().getResources());

        if (input.getSpec().getResources().getRequests() == null
                || Objects.equals(input.getSpec().getResources().getRequests().get("storage"), Quantity.parse("0"))) {
            throw new IllegalArgumentException("capacity cannot be empty for PVC '%s'".formatted(volSuffix));
        }

        if (input.getSpec().getAccessModes().isEmpty()) {
            throw new IllegalArgumentException("access-mode cannot be empty for PVC '%s'".formatted(volSuffix));
        }

        PersistentVolumeClaimSpec ps = new PersistentVolumeClaimSpecBuilder()
                .withAccessModes(input.getSpec().getAccessModes())
                .withResources(input.getSpec().getResources())
                .build();

        PersistentVolumeClaimBuilder builder = new PersistentVolumeClaimBuilder();
        return builder.editMetadata()
                .withName(input.getMetadata().getName())
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .withSpec(ps)
                .build();
    }
}
