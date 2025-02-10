// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * CheckOrbitConfigurationStep checks that orbit configuration are rolled out
 *
 * <p>It needs to inspect the CR and performs the necessary checks to ensure that the orbit configuration is deployed.
 */
public class CheckOrbitConfigurationStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {

    private final CustomResourceManager customResourceManager;

    public CheckOrbitConfigurationStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        return status; // FIXME implement the logic
    }
}
