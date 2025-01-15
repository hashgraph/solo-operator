// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/** CheckConsensusNodeRunningStep checks if the consensus node is ACTIVE and processing transactions */
public class CheckConsensusNodeRunningStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        throw new ProcessorException("Not implemented");
    }
}
