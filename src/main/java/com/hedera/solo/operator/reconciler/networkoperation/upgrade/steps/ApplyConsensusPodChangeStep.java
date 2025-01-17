// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * ApplyConsensusPodChangeStep applies the changeset to the consensus pods
 *
 * <p>It makes a single atomic patch to consensus-capsule CR using the following changesets. It will be checked by
 * {@link NetworkCheckProcessor} when deferred.
 *
 * <ol>
 *   <li>Apply consensus-node changeset (image source, nodeProperties and logging).
 *   <li>Apply streamUploader changesets (e.g., recordStreamUploader, backUploader, etc.).
 * </ol>
 *
 * <p>If this step is re-invoked, the changes in consensus-capsule CR should be idempotent.
 */
public class ApplyConsensusPodChangeStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        // TODO: Implement the logic to apply the changeset

        throw new ProcessorException("Not implemented");
    }
}
