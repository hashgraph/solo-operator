// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.envoy.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.render.QuteUtils.generate;
import static com.hedera.solo.operator.library.render.QuteUtils.template;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.labels;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.podDisruptionBudgetName;

import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import io.fabric8.kubernetes.api.model.policy.v1.PodDisruptionBudget;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.quarkus.qute.Template;

/** Represents a Kubernetes PodDisruptionBudget resource for EnvoyProxy. */
public class EnvoyPodDisruptionBudget extends CRUDKubernetesDependentResource<PodDisruptionBudget, EnvoyProxy> {
    Template template = template("envoy/envoy-pdb.yaml");

    @Override
    protected PodDisruptionBudget desired(final EnvoyProxy primary, final Context<EnvoyProxy> context) {
        return edit(generate(PodDisruptionBudget.class, template, t -> t.data("primary", primary)))
                .with(metadata -> metadata.editMetadata()
                        .withName(podDisruptionBudgetName(primary))
                        .addToLabels(labels(primary)))
                .with(spec -> spec.editSpec().editSelector().addToMatchLabels(labels(primary)))
                .build();
    }

    @Override
    protected ResourceID targetSecondaryResourceID(final EnvoyProxy primary, final Context<EnvoyProxy> context) {
        return new ResourceID(
                podDisruptionBudgetName(primary), primary.getMetadata().getNamespace());
    }
}
