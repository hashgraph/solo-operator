// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * CheckConsensusPodConfigurationStep checks that consensus pod configuration are rolled out
 *
 * <p>It needs to inspect the CR and performs the necessary checks to ensure that the consensus nodes are running with
 * correct version and configuration files.
 */
public class CheckConsensusPodConfigurationStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {

    private final CustomResourceManager customResourceManager;

    public CheckConsensusPodConfigurationStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        return status; // FIXME implement the logic
    }
}
