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

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;
import java.util.Objects;
import java.util.Optional;

@KubernetesDependent(resourceDiscriminator = CapsuleEventsStreamPvc.class)
public class CapsuleEventsStreamPvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule>
        implements ResourceDiscriminator<PersistentVolumeClaim, ConsensusCapsule> {

    public CapsuleEventsStreamPvc() {
        super(PersistentVolumeClaim.class);
    }

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

    public static class Activate implements Condition<PersistentVolumeClaim, ConsensusCapsule> {
        @Override
        public boolean isMet(
                DependentResource<PersistentVolumeClaim, ConsensusCapsule> dependentResource,
                ConsensusCapsule primary,
                Context<ConsensusCapsule> context) {
            return getInputPvcSpec(primary) != null;
        }
    }

    @Override
    public Optional<PersistentVolumeClaim> distinguish(
            Class<PersistentVolumeClaim> resource, ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        return context.getSecondaryResources(resource).stream()
                .filter((r) -> Objects.equals(
                        r.getMetadata().getName(),
                        CapsulePvcUtil.validPvcName(primary, VOLUME_EVENTS_STREAM_NAME_SUFFIX)))
                .findAny();
    }
}
