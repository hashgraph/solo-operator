// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.rbac.RoleBinding;
import io.fabric8.kubernetes.api.model.rbac.RoleBindingBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleRoleBinding extends CRUDKubernetesDependentResource<RoleBinding, ConsensusCapsule> {

    public static final String TEMPLATE = "role-binding.yaml";
    public static final String NAME_SUFFIX = "-rb";

    public CapsuleRoleBinding() {
        super(RoleBinding.class);
    }

    @Override
    protected RoleBinding desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        String rbName = primary.getMetadata().getName();
        final String namespace = primary.getMetadata().getNamespace();
        rbName =
                rbName.length() > (63 - NAME_SUFFIX.length()) ? rbName.substring(0, 63 - NAME_SUFFIX.length()) : rbName;

        String saName = primary.getMetadata().getName();
        saName = saName.length() > (63 - CapsuleServiceAccount.NAME_SUFFIX.length())
                ? saName.substring(0, 63 - CapsuleServiceAccount.NAME_SUFFIX.length())
                : saName;

        final String roleName = String.format("%s%s", rbName, NAME_SUFFIX);
        final String subjectName = String.format("%s%s", saName, CapsuleServiceAccount.NAME_SUFFIX);

        final RoleBinding rb = loadYaml(RoleBinding.class, getClass(), TEMPLATE);
        RoleBindingBuilder rbb = Metadata.applyStandardLabels(
                        rb.edit().editMetadata(),
                        rbName,
                        namespace,
                        Constants.COMPONENT_CONSENSUS,
                        Constants.COMPONENT_ROLE_BINDING,
                        null)
                .withName(roleName)
                .withNamespace(namespace)
                .endMetadata();

        var sb = rbb.editSubject(0);
        sb.withName(subjectName)
                .withNamespace(primary.getMetadata().getNamespace())
                .endSubject();

        return rbb.build();
    }
}
