// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_CONSENSUS_CAPSULE_NAME_KEY;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.rbac.RoleBinding;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;

@KubernetesDependent
public class CapsuleService extends CRUDKubernetesDependentResource<Service, ConsensusCapsule> {

    public static final String TEMPLATE = "service.yaml";
    public static final String NAME_SUFFIX = "-svc";

    public CapsuleService() {
        super(Service.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Service>()
                .withUseSSA(Boolean.FALSE)
                .withLabelSelector("app=consensus-node")
                .build());
    }

    @Override
    protected Service desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);

        return loadYaml(Service.class, getClass(), TEMPLATE)
                .edit()
                .editMetadata()
                .addToLabels(LABEL_CONSENSUS_CAPSULE_NAME_KEY, metadata.getName())
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .editSpec()
                .addToSelector(LABEL_CONSENSUS_CAPSULE_NAME_KEY, metadata.getName())
                .endSpec()
                .build();
    }
}
