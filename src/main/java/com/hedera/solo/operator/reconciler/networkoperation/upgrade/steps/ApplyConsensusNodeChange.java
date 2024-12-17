// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.AbstractStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import io.javaoperatorsdk.operator.api.reconciler.Context;

public class ApplyConsensusNodeChange extends AbstractStep<NetworkOperation> {
    @Override
    public NetworkOperation execute(NetworkOperation resource, Context<NetworkOperation> context) {
        throw new UnsupportedOperationException("Not implemented");
    }
}
