// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ServiceAccount;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleServiceAccount extends CRUDKubernetesDependentResource<ServiceAccount, ConsensusCapsule> {
    public static final String TEMPLATE = "sa.yaml";
    public static final String NAME_SUFFIX = "-sa";

    public CapsuleServiceAccount() {
        super(ServiceAccount.class);
    }

    @Override
    protected ServiceAccount desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        String name = primary.getMetadata().getName();
        name = name.length() > (63 - NAME_SUFFIX.length()) ? name.substring(0, 63 - NAME_SUFFIX.length()) : name;

        final String namespace = primary.getMetadata().getNamespace();
        final String orbit = primary.getSpec().getOrbit();

        final String saName = String.format("%s%s", name, NAME_SUFFIX);

        final ServiceAccount sa = loadYaml(ServiceAccount.class, getClass(), TEMPLATE);
        return Metadata.applyStandardLabels(
                        sa.edit().editMetadata(),
                        saName,
                        orbit,
                        Constants.COMPONENT_CONSENSUS_NODE,
                        Constants.COMPONENT_SERVICE_ACCOUNT,
                        null)
                .withName(saName)
                .withNamespace(namespace)
                .endMetadata()
                .build();
    }
}
