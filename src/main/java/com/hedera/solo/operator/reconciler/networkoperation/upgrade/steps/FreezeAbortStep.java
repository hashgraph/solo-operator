// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.generic.Nullable.orEmptyList;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.common.ConditionStatus.True;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.stream.IntStream;

public class FreezeAbortStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {

    public static final String TYPE = "FreezeAbort";

    @Override
    public NetworkOperationStatus run(
            final NetworkOperation resource,
            final NetworkOperationStatus status,
            final Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        List<Condition> conditions = new ArrayList<>(orEmptyList(() -> status.getConditions()));
        int index = IntStream.range(0, conditions.size())
                .filter(i -> TYPE.equals(conditions.get(i).getType()))
                .findFirst()
                .orElse(-1);
        if (index == -1) {
            conditions.add(
                    new Condition(timestamp(), null, resource.getMetadata().getGeneration(), null, True, TYPE));
        } else {
            conditions.set(
                    index,
                    conditions
                            .get(index)
                            .edit()
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .build());
        }
        return status.edit()
                .withPhase(NetworkOperationStatus.Phase.Succeeded)
                .withConditions(conditions)
                .build();
    }
}
