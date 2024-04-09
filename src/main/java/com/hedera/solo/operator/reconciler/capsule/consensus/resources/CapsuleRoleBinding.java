package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.rbac.RoleBinding;
import io.fabric8.kubernetes.api.model.rbac.RoleBindingBuilder;
import io.fabric8.kubernetes.api.model.rbac.RoleBindingFluent;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

@KubernetesDependent
public class CapsuleRoleBinding extends CRUDKubernetesDependentResource<RoleBinding, ConsensusCapsule> {

    public static final String TEMPLATE = "role-binding.yaml";
    public static final String NAME_SUFFIX = "-rb";

    public CapsuleRoleBinding() {
        super(RoleBinding.class);
    }

    @Override
    protected RoleBinding desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        final RoleBindingBuilder rb = loadYaml(RoleBinding.class, getClass(), TEMPLATE).edit();

        String rbName = primary.getMetadata().getName();
        rbName = rbName.length() > (63 - NAME_SUFFIX.length()) ? rbName.substring(0, 63 - NAME_SUFFIX.length()) : rbName;

        String saName = primary.getMetadata().getName();
        saName = saName.length() > (63 - CapsuleServiceAccount.NAME_SUFFIX.length()) ? saName.substring(0, 63 - CapsuleServiceAccount.NAME_SUFFIX.length()) : saName;

        rb.editMetadata()
                .withName(String.format("%s%s", rbName, NAME_SUFFIX))
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata();

//        var sb = rb.addNewSubject();
//        if (rb.hasSubjects()) {
            var sb = rb.editSubject(0);
//        }

        sb.withName(String.format("%s%s", saName, CapsuleServiceAccount.NAME_SUFFIX))
                .withNamespace(primary.getMetadata().getNamespace())
                .endSubject();

        return rb.build();
    }
}
