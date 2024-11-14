// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.rbac.Role;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Optional;

@KubernetesDependent
public class ConsensusHeadlessService extends CRUDNoGCKubernetesDependentResource<Service, Orbit>
        implements ResourceDiscriminator<Service, Orbit> {
    public static final String TEMPLATE = "headless-service.yaml";

    public ConsensusHeadlessService() {
        super(Service.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Service>()
                .withLabelSelector(LABEL_SELECTOR_CONSENSUS_NODE)
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    protected Service desired(Orbit primary, Context<Orbit> context) {
        return Metadata.applyOrbitMetaTo(
                        primary,
                        loadYaml(Service.class, getClass(), TEMPLATE).edit().editMetadata())
                .withNamespace(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }

    @Override
    public Optional<Service> distinguish(Class<Service> resource, Orbit primary, Context<Orbit> context) {
        return context.getSecondaryResources(Service.class).stream()
                .filter(service -> "None".equalsIgnoreCase(service.getSpec().getClusterIP()))
                .findAny();
    }
}
