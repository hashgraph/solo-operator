// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.envoy.resources;

import static com.hedera.solo.operator.library.generic.Nullable.orNull;
import static com.hedera.solo.operator.library.model.Labels.recommendedLabels;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import java.util.Map;

/** Utility class for handling EnvoyProxy resources. */
public class EnvoyUtils {

    /** Private constructor to prevent instantiation. */
    private EnvoyUtils() {}

    /**
     * Returns the name of the Envoy config map based on the EnvoyProxy resource.
     *
     * @param primary the EnvoyProxy resource
     * @return the config map name
     */
    public static String configMapName(final EnvoyProxy primary) {
        return validName(primary.getMetadata().getName(), "-cm");
    }

    /**
     * Returns the name of the Envoy deployment based on the EnvoyProxy resource.
     *
     * @param primary the EnvoyProxy resource
     * @return the deployment name
     */
    public static String deploymentName(final EnvoyProxy primary) {
        return validName(primary.getMetadata().getName(), "-deploy");
    }

    /**
     * Returns the name of the Envoy pod disruption budget based on the EnvoyProxy resource.
     *
     * @param primary the EnvoyProxy resource
     * @return the pod disruption budget name
     */
    public static String podDisruptionBudgetName(final EnvoyProxy primary) {
        return validName(primary.getMetadata().getName(), "-pdb");
    }

    /**
     * Returns the name of the Envoy service based on the EnvoyProxy resource.
     *
     * @param primary the EnvoyProxy resource
     * @return the service name
     */
    public static String serviceName(final EnvoyProxy primary) {
        return validName(primary.getMetadata().getName(), "-svc");
    }

    /**
     * Returns a map of labels for the EnvoyProxy resource.
     *
     * @param primary the EnvoyProxy resource
     * @return a map of labels
     */
    public static Map<String, String> labels(final EnvoyProxy primary) {
        return recommendedLabels(
                primary, "proxy", orNull(() -> primary.getSpec().getImage()));
    }
}
