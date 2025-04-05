// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

@ApplicationScoped
public class NetworkGenesisProcessor
        implements Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>> {
    private final NetworkGenesisTask networkGenesisTask;

    @Inject
    public NetworkGenesisProcessor(NetworkGenesisTask networkGenesisTask) {
        this.networkGenesisTask = networkGenesisTask;
    }

    @Override
    public ProcessorResult<NetworkOperationStatus> process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        return networkGenesisTask.execute(resource, context);
    }
}
