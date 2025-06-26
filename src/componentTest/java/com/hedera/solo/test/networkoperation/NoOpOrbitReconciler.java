// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;

/**
 * This class implements a NoOpOrbitReconciler, which is a simple Reconciler for Orbit resources. It does not perform
 * any actual reconciliation actions and simply returns no update control.
 */
@ControllerConfiguration
public class NoOpOrbitReconciler implements Reconciler<Orbit> {
    @Override
    public UpdateControl<Orbit> reconcile(final Orbit resource, final Context<Orbit> context) {
        return UpdateControl.noUpdate();
    }
}
