// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;

public class ApplyOrbitConfigurationChangeStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public ApplyOrbitConfigurationChangeStep() {}

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        // TODO: Implement the logic to apply the orbit level configuration changes

        throw new ProcessorException("Not implemented");
    }
}
