/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
        final String nodeId = Metadata.extractNodeIdFromConsensusCapsule(cc);
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

        PersistentVolumeClaimBuilder builder = new PersistentVolumeClaimBuilder();
        return builder.editMetadata()
                .withName(CapsulePvcUtil.validPvcName(primary, volSuffix))
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .withSpec(ps)
                .build();
    }
}
