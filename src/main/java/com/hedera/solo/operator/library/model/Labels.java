// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.model;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.util.Map;
import java.util.Objects;
import java.util.function.Predicate;

public class Labels {
    private Labels() {}

    public static Predicate<HasMetadata> hasLabel(final String labelName, final String labelValue) {
        return hasLabels(Map.of(labelName, labelValue));
    }

    public static Predicate<HasMetadata> hasLabels(final Map<String, String> labels) {
        return hasMetadata -> labels.entrySet().stream().allMatch(e -> hasLabel(hasMetadata, e.getKey(), e.getValue()));
    }

    public static boolean hasLabel(final HasMetadata hasMetadata, final String labelName, final String labelValue) {
        return Objects.equals(hasMetadata.getMetadata().getLabels().get(labelName), labelValue);
    }
}
