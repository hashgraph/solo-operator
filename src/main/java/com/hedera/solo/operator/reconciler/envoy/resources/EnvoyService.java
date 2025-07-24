// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.envoy.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.render.QuteUtils.generate;
import static com.hedera.solo.operator.library.render.QuteUtils.template;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.labels;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.serviceName;

import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.quarkus.qute.Template;

/** Represents a Kubernetes Service resource for EnvoyProxy. */
public class EnvoyService extends CRUDKubernetesDependentResource<Service, EnvoyProxy> {
    Template template = template("envoy/envoy-svc.yaml");

    @Override
    protected Service desired(final EnvoyProxy primary, final Context<EnvoyProxy> context) {
        return edit(generate(Service.class, template, t -> t.data("primary", primary)))
                .with(metadata ->
                        metadata.editMetadata().withName(serviceName(primary)).addToLabels(labels(primary)))
                .with(spec -> spec.editSpec().withSelector(labels(primary)))
                .build();
    }
}
