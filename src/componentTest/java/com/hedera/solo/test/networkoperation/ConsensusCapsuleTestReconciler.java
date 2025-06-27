// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import java.util.Objects;

/**
 * This class is a test reconciler for ConsensusCapsule custom resources. It sets the phase of the ConsensusCapsule
 * based on a label "fail". If the "fail" label is true, it sets the phase to Failed, otherwise it sets it to Running.
 */
@ControllerConfiguration
public class ConsensusCapsuleTestReconciler implements Reconciler<ConsensusCapsule> {
    @Override
    public UpdateControl<ConsensusCapsule> reconcile(
            final ConsensusCapsule resource, final Context<ConsensusCapsule> context) {
        ConsensusCapsuleStatus.Phase phase;
        if (Objects.equals(resource.getMetadata().getLabels().get("fail"), "true")) {
            phase = ConsensusCapsuleStatus.Phase.Failed;
        } else {
            phase = ConsensusCapsuleStatus.Phase.Running;
        }
        return UpdateControl.patchStatus(edit(new ConsensusCapsule())
                .with(metadata -> metadata.editMetadata()
                        .withName(resource.getMetadata().getName())
                        .withNamespace(resource.getMetadata().getNamespace()))
                .with(status -> status.editStatus().withPhase(phase))
                .build());
    }
}
