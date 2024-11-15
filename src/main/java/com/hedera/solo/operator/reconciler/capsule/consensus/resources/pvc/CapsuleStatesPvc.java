// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources.pvc;

import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_STATES_NAME_SUFFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;
import java.util.Objects;
import java.util.Optional;

@KubernetesDependent
public class CapsuleStatesPvc extends CRUDKubernetesDependentResource<PersistentVolumeClaim, ConsensusCapsule>
        implements ResourceDiscriminator<PersistentVolumeClaim, ConsensusCapsule> {
    public CapsuleStatesPvc() {
        super(PersistentVolumeClaim.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<PersistentVolumeClaim>()
                .withResourceDiscriminator(this)
                .build());
    }

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

    public static class Ready implements Condition<PersistentVolumeClaim, ConsensusCapsule> {
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
                        r.getMetadata().getName(), CapsulePvcUtil.validPvcName(primary, VOLUME_STATES_NAME_SUFFIX)))
                .findAny();
    }
}
