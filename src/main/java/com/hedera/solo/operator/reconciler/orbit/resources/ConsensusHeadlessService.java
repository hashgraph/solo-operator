// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.ServiceBuilder;
import io.javaoperatorsdk.operator.api.config.informer.Informer;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

/**
 * The `ConsensusHeadlessService` class is a Kubernetes dependent resource that manages the creation and update of a
 * headless service for consensus nodes in an Orbit CRD.
 */
@KubernetesDependent(informer = @Informer(labelSelector = Constants.LABEL_SELECTOR_CONSENSUS_NODE_APP))
public class ConsensusHeadlessService extends CRUDNoGCKubernetesDependentResource<Service, Orbit> {
    public static final String TEMPLATE = "headless-service.yaml";
    public static final String NAME = "consensus";

    /** Default constructor initializing the resource type to Service. */
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
}
