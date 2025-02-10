// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/** CheckStreamUploaderRunningStep checks that a particular stream uploader is in running state */
public class CheckStreamUploaderRunningStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    private final String containerName;

    private final CustomResourceManager customResourceManager;

    public CheckStreamUploaderRunningStep(CustomResourceManager customResourceManager, String name) {
        this.customResourceManager = customResourceManager;
        this.containerName = name;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        // FIXME: Implement this method
        return status;
    }
}
