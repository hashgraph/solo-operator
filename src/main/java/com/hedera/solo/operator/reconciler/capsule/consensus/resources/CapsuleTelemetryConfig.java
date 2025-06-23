// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_OPEN_TELEMETRY_COLLECTOR;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_ORBIT_NAME;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.OpenTelemetryCollector;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.utils.Serialization;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.dependent.DependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Condition;
import io.quarkus.qute.Location;
import io.quarkus.qute.Template;
import io.quarkus.qute.TemplateInstance;
import java.util.Objects;
import java.util.Optional;

/**
 * The CapsuleTelemetryConfig class is a Kubernetes dependent resource that manages the ConfigMap for OpenTelemetry
 * collector configuration.
 */
@KubernetesDependent
public class CapsuleTelemetryConfig extends CRUDKubernetesDependentResource<ConfigMap, ConsensusCapsule> {

    @Location("telemetry/otel-collector-config.yaml")
    Template tpl;

    public static final String NAME_SUFFIX = "otel-collector-config";

    /** Constructor for CapsuleTelemetryConfig class, initializes the dependent resource with ConfigMap type. */
    public CapsuleTelemetryConfig() {
        super(ConfigMap.class);
    }

    /**
     * Generates a valid ConfigMap name based on the ConsensusCapsule primary resource.
     *
     * @param primary The ConsensusCapsule primary resource.
     * @return A string representing the valid ConfigMap name
     */
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

    /**
     * Retrieves the input OpenTelemetryCollector specification from the ConsensusCapsule primary resource.
     *
     * @param primary The ConsensusCapsule primary resource.
     * @return An Optional containing the OpenTelemetryCollector if present, otherwise empty
     */
    private static Optional<OpenTelemetryCollector> getInputOtelCollector(final ConsensusCapsule primary) {
        if (primary.getSpec().getPodProperties() == null) {
            return Optional.empty();
        }

        return Optional.ofNullable(
                primary.getSpec().getPodProperties().getContainers().getOpenTelemetryCollector());
    }

    /**
     * Renders the Qute template to generate a ConfigMap for OpenTelemetry collector configuration.
     *
     * @param primary The ConsensusCapsule primary resource.
     * @param context The operator context.
     * @return A ConfigMap instance rendered from the Qute template
     */
    protected ConfigMap renderTemplate(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        final String orbitName = primary.getMetadata().getName();
        final Optional<OpenTelemetryCollector> oc = getInputOtelCollector(primary);

        final TemplateInstance templateInstance = tpl.instance();
        templateInstance.data(TPL_VAR_ORBIT_NAME, orbitName);
        templateInstance.data(TPL_VAR_OPEN_TELEMETRY_COLLECTOR, oc.orElse(new OpenTelemetryCollector()));

        return Serialization.unmarshal(templateInstance.render(), ConfigMap.class);
    }

    /**
     * Condition class to check if the desired ConfigMap is ready for use in the ConsensusCapsule reconciliation
     * process.
     */
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
}
