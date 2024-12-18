// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.node;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import io.javaoperatorsdk.operator.api.reconciler.Context;

public class NodeOperationProcessor implements Processor<NetworkOperation, ProcessorResult> {
    @Override
    public ProcessorResult process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        return null;
    }
}
