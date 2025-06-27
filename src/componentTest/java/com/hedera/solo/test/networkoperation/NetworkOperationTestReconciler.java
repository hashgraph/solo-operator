// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationSequenceReconciler;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.NetworkGenesisProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.node.NodeOperationProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.NetworkUpgradeProcessor;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

/**
 * The NetworkOperationTestReconciler class is a custom reconciler for managing NetworkOperation resources. It extends
 * the NetworkOperationSequenceReconciler to handle sequence-based processing of network operations.
 */
@ControllerConfiguration
public class NetworkOperationTestReconciler extends NetworkOperationSequenceReconciler {

    /**
     * Constructs a new NetworkOperationTestReconciler with specified custom resource manager, processors for network
     * genesis, upgrade, check, and node operation.
     *
     * @param customResourceManager the CustomResourceManager to manage Kubernetes resources
     * @param networkGenesisProcessor the processor for handling network genesis operations
     * @param networkUpgradeProcessor the processor for handling network upgrade operations
     * @param networkCheckProcessor the processor for checking network conditions
     * @param nodeOperationProcessor the processor for managing node operations
     */
    public NetworkOperationTestReconciler(
            CustomResourceManager customResourceManager,
            NetworkGenesisProcessor networkGenesisProcessor,
            NetworkUpgradeProcessor networkUpgradeProcessor,
            NetworkCheckProcessor networkCheckProcessor,
            NodeOperationProcessor nodeOperationProcessor) {
        super(
                customResourceManager,
                networkGenesisProcessor,
                networkUpgradeProcessor,
                networkCheckProcessor,
                nodeOperationProcessor);
    }

    @Override
    public UpdateControl<NetworkOperation> reconcile(
            final NetworkOperation resource, final Context<NetworkOperation> context) throws Exception {
        if (null != resource.getStatus()
                && "true".equals(resource.getMetadata().getLabels().get("no-update"))) {
            return UpdateControl.noUpdate();
        }
        return super.reconcile(resource, context);
    }
}
