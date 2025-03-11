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

@ControllerConfiguration
public class NetworkOperationTestReconciler extends NetworkOperationSequenceReconciler {
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
