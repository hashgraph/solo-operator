// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.node.tasks;

import com.hedera.solo.operator.library.processor.AbstractProcessorTask;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.node.steps.NodeAddStep;
import java.time.Duration;
import java.util.List;

public class NodeDeleteTask extends AbstractProcessorTask<NetworkOperation, NetworkOperationStatus> {
    public static final Duration RECONCILE_DELAY = Duration.ofSeconds(1);

    @Override
    public List<ProcessorStep<NetworkOperation, NetworkOperationStatus>> steps() throws ProcessorException {
        return List.of(new NodeAddStep());
    }

    @Override
    protected Duration computeReconcileDelay(NetworkOperation resource, NetworkOperationStatus status)
            throws ProcessorException {
        return RECONCILE_DELAY;
    }
}
