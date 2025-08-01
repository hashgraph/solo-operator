// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.model;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.CustomResource;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Objects;
import java.util.function.Predicate;
import java.util.stream.Collectors;

/** The {@code Labels} class provides utility methods for working with Kubernetes labels. */
public class Labels {
    /** Private constructor to prevent instantiation of the utility class. */
    private Labels() {}

    /**
     * Creates a predicate that checks if a {@link HasMetadata} object has a specific label.
     *
     * @param labelName The name of the label to check.
     * @param labelValue The value of the label to check.
     * @return A predicate that evaluates whether the metadata contains the specified label
     */
    public static Predicate<HasMetadata> hasLabel(final String labelName, final String labelValue) {
        return hasLabels(Map.of(labelName, labelValue));
    }

    /**
     * Creates a predicate that checks if a {@link HasMetadata} object has multiple specific labels.
     *
     * @param labels A map of label names and their corresponding values to check.
     * @return A predicate that evaluates whether the metadata contains all specified labels
     */
    public static Predicate<HasMetadata> hasLabels(final Map<String, String> labels) {
        return hasMetadata -> labels.entrySet().stream().allMatch(e -> hasLabel(hasMetadata, e.getKey(), e.getValue()));
    }

    /**
     * Checks if a {@link HasMetadata} object has a specific label with the given name and value.
     *
     * @param hasMetadata The metadata object to check for the label.
     * @param labelName The name of the label to check.
     * @param labelValue The expected value of the label.
     * @return {@code true} if the metadata contains the specified label with the given value, otherwise {@code false}
     */
    public static boolean hasLabel(final HasMetadata hasMetadata, final String labelName, final String labelValue) {
        return Objects.equals(hasMetadata.getMetadata().getLabels().get(labelName), labelValue);
    }

    /**
     * Converts a name into a valid Solo Operator domain format.
     *
     * @param name The name to be included in the domain.
     * @return A string representing the Solo Operator domain with the specified name
     */
    public static String soloOperatorDomain(final String name) {
        return "operator.solo.hedera.com/" + name;
    }

    /**
     * Constructs a map of recommended labels for a custom resource.
     *
     * @param primary The primary custom resource for which the labels are being created.
     * @param component The component within the application (e.g., frontend, backend, database).
     * @param version The current version of the application (if applicable).
     * @return A map containing recommended labels for the custom resource
     */
    public static Map<String, String> recommendedLabels(
            final CustomResource<?, ?> primary, final String component, final String version) {
        final Map<String, String> map = new LinkedHashMap<>();
        // The name of the application (CRD type)
        map.put("name", primary.getSingular());
        // The name of the application instance (CRD name)
        if (primary.getMetadata() != null && primary.getMetadata().getName() != null) {
            map.put("instance", primary.getMetadata().getName());
        }
        // The current version of the application (if applicable)
        if (version != null) {
            String semver = version;
            int idx = semver.lastIndexOf(':');
            if (idx != -1) {
                semver = semver.substring(idx + 1);
            }
            if (!semver.isBlank()) {
                map.put("version", semver);
            }
        }
        // The component within the application (e.g., frontend, backend, database)
        map.put("component", component);
        // The name of a higher level application this is part of
        map.put("part-of", "hiero-stack");
        // The tool being used to manage the application
        map.put("managed-by", "solo-operator");
        return map.entrySet().stream()
                .collect(Collectors.toMap(e -> "app.kubernetes.io/" + e.getKey(), Map.Entry::getValue));
    }
}
