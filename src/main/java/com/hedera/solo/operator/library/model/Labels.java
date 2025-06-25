// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.model;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.util.Map;
import java.util.Objects;
import java.util.function.Predicate;

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
}
