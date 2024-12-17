// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.AbstractStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * Revalidate the selected image for the consensus node still exists, if not, choose the next available image. If no
 * images are available, fail the operation.
 */
public class ReverifyConsensusNodeChange extends AbstractStep<NetworkOperation> {
    @Override
    public NetworkOperation execute(NetworkOperation resource, Context<NetworkOperation> context) {
        throw new UnsupportedOperationException("Not implemented");
    }
}
