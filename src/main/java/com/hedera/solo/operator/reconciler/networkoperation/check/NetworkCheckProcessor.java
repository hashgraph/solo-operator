// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

/** Checks that the network is in a valid state. */
@ApplicationScoped
public class NetworkCheckProcessor
        implements Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>> {
    @Inject
    NetworkCheckTask networkGenesisTask;

    @Override
    public ProcessorResult<NetworkOperationStatus> process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        return networkGenesisTask.execute(resource, context);
    }
}
