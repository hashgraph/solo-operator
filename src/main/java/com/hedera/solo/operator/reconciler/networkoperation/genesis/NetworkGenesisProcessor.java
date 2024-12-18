// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.tasks.NetworkGenesisTask;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

@ApplicationScoped
public class NetworkGenesisProcessor implements Processor<NetworkOperation, ProcessorResult> {
    @Inject
    NetworkGenesisTask networkGenesisTask;

    @Override
    public ProcessorResult process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        NetworkOperationStatus status = networkGenesisTask.execute(resource, context);
        return new ProcessorResult(status, NetworkGenesisTask.RETRY_DELAY, NetworkGenesisTask.MAX_RETRY);
    }
}
