// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis.tasks;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.library.processor.ProcessorTask;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import java.time.Duration;
import java.util.List;

@ApplicationScoped
public class NetworkGenesisTask implements ProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RETRY_DELAY = Duration.ofSeconds(1);
    public static final long MAX_RETRY = ProcessorResult.INFINITE_RETRY;

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(new ProcessorStep<NetworkOperation, NetworkOperationStatus>() {
            @Override
            public NetworkOperationStatus run(
                    NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
                    throws ProcessorException {
                throw new ProcessorException("Not implemented");
            }
        });
    }
}
