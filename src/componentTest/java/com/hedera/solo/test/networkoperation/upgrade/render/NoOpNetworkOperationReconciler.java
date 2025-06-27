// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

/**
 * The NoOpNetworkOperationReconciler class is a no-op reconciler for NetworkOperation resources. It does not perform
 * any updates or patches to the NetworkOperation resource during reconciliation.
 */
@ControllerConfiguration
public class NoOpNetworkOperationReconciler implements Reconciler<NetworkOperation> {
    @Override
    public UpdateControl<NetworkOperation> reconcile(
            final NetworkOperation resource, final Context<NetworkOperation> context) {
        return UpdateControl.noUpdate();
    }
}
