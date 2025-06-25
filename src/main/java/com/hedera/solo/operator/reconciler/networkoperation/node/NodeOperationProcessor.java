// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.node;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;

/**
 * The NodeOperationProcessor class is an implementation of the Processor interface for handling network operations
 * related to nodes.
 */
@ApplicationScoped
public class NodeOperationProcessor
        implements Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>> {
    @Override
    public ProcessorResult<NetworkOperationStatus> process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        throw new ProcessorException("Not implemented");
    }
}
