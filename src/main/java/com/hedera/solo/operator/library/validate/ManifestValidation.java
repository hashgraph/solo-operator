// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.validate;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.HasMetadata;
import java.lang.invoke.MethodHandles;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import java.util.function.Consumer;

/**
 * The {@code ManifestValidation} class provides a static method for validating Kubernetes resources using custom
 * conditions.
 */
public class ManifestValidation {
    public static final String name = MethodHandles.lookup().lookupClass().getSimpleName();

    /** Private constructor to prevent instantiation of this utility class. */
    private ManifestValidation() {}

    /**
     * Validates the given resource using multiple validators and updates or adds a condition if necessary.
     *
     * @param <T> The type of the Kubernetes resource implementing {@code HasMetadata}.
     * @param resource The Kubernetes resource to be validated.
     * @param conditions A list of existing conditions, which may be updated by this method.
     * @param validators One or more validation functions that implement {@code Consumer<Condition>}.
     * @return A list of conditions reflecting the result of the validation process
     */
    @SafeVarargs
    public static <T extends HasMetadata> List<Condition> validate(
            final T resource, final List<Condition> conditions, final Consumer<Condition>... validators) {
        List<Condition> list =
                Optional.ofNullable(conditions).map(ArrayList::new).orElseGet(ArrayList::new);
        Condition condition =
                list.stream().filter(c -> name.equals(c.getType())).findAny().orElse(null);
        if (condition == null) {
            condition = new ConditionBuilder().withType(name).build();
            list.add(condition);
        }
        Condition copy = condition.edit().build();
        condition.setObservedGeneration(resource.getMetadata().getGeneration());
        condition.setStatus(ConditionStatus.True);
        condition.setReason(null);
        condition.setMessage(null);
        for (Consumer<Condition> validator : validators) {
            validator.accept(condition);
            if (!Objects.equals(condition.getStatus(), ConditionStatus.True)) {
                break;
            }
        }
        if (!Objects.equals(condition, copy)) {
            condition.setLastTransitionTime(timestamp());
        }
        return list;
    }
}
