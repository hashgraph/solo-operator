// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.ServiceBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Optional;

@KubernetesDependent(
        labelSelector = Constants.LABEL_SELECTOR_CONSENSUS_NODE,
        resourceDiscriminator = ConsensusHeadlessService.class)
public class ConsensusHeadlessService extends CRUDNoGCKubernetesDependentResource<Service, Orbit>
        implements ResourceDiscriminator<Service, Orbit> {
    public static final String TEMPLATE = "headless-service.yaml";
    public static final String NAME = "consensus";

    public ConsensusHeadlessService() {
        super(Service.class);
    }

    @Override
    protected Service desired(Orbit primary, Context<Orbit> context) {
        final String orbitName = primary.getMetadata().getName();
        final Service svc = loadYaml(Service.class, getClass(), TEMPLATE);
        final ServiceBuilder sb =
                Metadata.applyOrbitMetaTo(primary, svc.edit().editMetadata()).endMetadata();
        return Metadata.applyStandardLabels(
                        sb.editMetadata(),
                        NAME,
                        orbitName,
                        Constants.COMPONENT_ORBIT,
                        Constants.COMPONENT_SERVICE,
                        null)
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
