// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Map;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, labelSelector = "app=consensus-node")
public class CapsuleService extends CRUDKubernetesDependentResource<Service, ConsensusCapsule> {

    public static final String TEMPLATE = "service.yaml";
    public static final String NAME_SUFFIX = "-svc";

    public CapsuleService() {
        super(Service.class);
    }

    @Override
    protected Service desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final ObjectMeta metadata = primary.getMetadata();
        final String validName = validName(metadata.getName(), NAME_SUFFIX);
        final String namespace = metadata.getNamespace();
        final String orbit = primary.getSpec().getOrbit();
        final String nodeName = Metadata.prepareNodeName(primary.getSpec().getNodeId());
        final Map<String, String> labels = CapsulePod.prepareLabels(primary);
        final Service service = loadYaml(Service.class, getClass(), TEMPLATE);

        return Metadata.applyStandardLabels(
                        service.edit().editMetadata(),
                        validName,
                        orbit,
                        Constants.COMPONENT_CONSENSUS,
                        Constants.COMPONENT_SERVICE,
                        labels)
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata()
                .editSpec()
                .addToSelector(Constants.LABEL_NODE_NAME, nodeName)
                .endSpec()
                .build();
    }
}
