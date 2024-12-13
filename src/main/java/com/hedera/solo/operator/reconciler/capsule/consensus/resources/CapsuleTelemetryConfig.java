// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_OPEN_TELEMETRY_COLLECTOR;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_ORBIT_NAME;

import com.hedera.solo.operator.model.commonspec.OpenTelemetryCollector;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.utils.Serialization;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;
import io.quarkus.qute.Location;
import io.quarkus.qute.Template;
import io.quarkus.qute.TemplateInstance;
import java.util.Objects;
import java.util.Optional;

@KubernetesDependent(resourceDiscriminator = CapsuleTelemetryConfig.Discriminator.class)
public class CapsuleTelemetryConfig extends CRUDNoGCKubernetesDependentResource<ConfigMap, ConsensusCapsule> {

    @Location("telemetry/otel-collector-config.yaml")
    Template tpl;

    public static final String NAME_SUFFIX = "otel-collector-config";

    public CapsuleTelemetryConfig() {
        super(ConfigMap.class);
    }

    private static String validConfigMapName(final ConsensusCapsule primary) {
        return "%s-%s".formatted(Metadata.prepareNodeName(primary), NAME_SUFFIX);
    }

    @Override
    protected ConfigMap desired(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        Objects.requireNonNull(primary.getMetadata(), "metadata is required");
        final String orbitName = primary.getMetadata().getName();
        final String namespace = primary.getMetadata().getNamespace();
        final String validName = validConfigMapName(primary);
        final ConfigMap cm = renderTemplate(primary, context);
        return Metadata.applyStandardLabels(
                        cm.edit().editMetadata(),
                        validName,
                        orbitName,
                        Constants.COMPONENT_ORBIT,
                        Constants.COMPONENT_CONFIG_MAP,
                        null)
                .withName(validName)
                .withNamespace(namespace)
                .endMetadata()
                .build();
    }

    private static Optional<OpenTelemetryCollector> getInputOtelCollector(final ConsensusCapsule primary) {
        if (primary.getSpec().getPodProperties() == null) {
            return Optional.empty();
        }

        return Optional.ofNullable(
                primary.getSpec().getPodProperties().getContainers().getOpenTelemetryCollector());
    }

    protected ConfigMap renderTemplate(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        final String orbitName = primary.getMetadata().getName();
        final Optional<OpenTelemetryCollector> oc = getInputOtelCollector(primary);

        final TemplateInstance templateInstance = tpl.instance();
        templateInstance.data(TPL_VAR_ORBIT_NAME, orbitName);
        templateInstance.data(TPL_VAR_OPEN_TELEMETRY_COLLECTOR, null);

        oc.ifPresent(o -> templateInstance.data(TPL_VAR_OPEN_TELEMETRY_COLLECTOR, o));

        return Serialization.unmarshal(templateInstance.render(), ConfigMap.class);
    }

    public static class Ready implements Condition<ConfigMap, ConsensusCapsule> {
        @Override
        public boolean isMet(
                DependentResource<ConfigMap, ConsensusCapsule> dependentResource,
                ConsensusCapsule primary,
                Context<ConsensusCapsule> context) {
            final Optional<OpenTelemetryCollector> oc = getInputOtelCollector(primary);
            return oc.isPresent() && oc.get().getEnabled();
        }
    }

    public static class Discriminator implements ResourceDiscriminator<ConfigMap, ConsensusCapsule> {
        @Override
        public Optional<ConfigMap> distinguish(
                final Class<ConfigMap> resource,
                final ConsensusCapsule primary,
                final Context<ConsensusCapsule> context) {
            final String namespace = primary.getMetadata().getNamespace();
            final String validName = validConfigMapName(primary);
            return context.getSecondaryResources(resource).stream()
                    .filter((r) -> Objects.equals(r.getMetadata().getName(), validName)
                            && Objects.equals(r.getMetadata().getNamespace(), namespace))
                    .findAny();
        }
    }
}
