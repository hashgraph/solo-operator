// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.envoy.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.data.AsciiArmor.base64Encode;
import static com.hedera.solo.operator.library.render.QuteUtils.generate;
import static com.hedera.solo.operator.library.render.QuteUtils.template;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.configMapName;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.labels;

import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.quarkus.qute.Template;
import java.util.Map;
import java.util.stream.Collectors;

/** Represents a Kubernetes ConfigMap resource for EnvoyProxy. */
public class EnvoyConfigMap extends CRUDKubernetesDependentResource<ConfigMap, EnvoyProxy> {
    Template template = template(getClass(), "envoy/envoy-cm.yaml");

    @Override
    protected ConfigMap desired(final EnvoyProxy primary, final Context<EnvoyProxy> context) {
        return edit(generate(ConfigMap.class, template, t -> t.data("primary", primary)))
                .with(metadata ->
                        metadata.editMetadata().withName(configMapName(primary)).addToLabels(labels(primary)))
                .with(data -> data.addToData(primary.getSpec().getFiles().entrySet().stream()
                        .collect(Collectors.toMap(e -> base64Encode(e.getKey()), Map.Entry::getValue))))
                .build();
    }

    @Override
    protected ResourceID targetSecondaryResourceID(final EnvoyProxy primary, final Context<EnvoyProxy> context) {
        return new ResourceID(configMapName(primary), primary.getMetadata().getNamespace());
    }
}
