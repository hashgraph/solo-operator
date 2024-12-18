// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.node.tasks;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.library.processor.ProcessorTask;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.List;

public class NodeUpdateTask implements ProcessorTask<NetworkOperation, NetworkOperationStatus> {
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
