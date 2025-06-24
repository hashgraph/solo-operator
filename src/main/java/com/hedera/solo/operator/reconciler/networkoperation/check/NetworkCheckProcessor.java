// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check;

import com.hedera.solo.operator.library.processor.Processor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;

/**
 * NetworkCheckProcessor checks that the network is in a valid state.
 *
 * <p>Other processors task usually defer to this if those tasks have finished processing and need to ensure that the
 * network is in valid state. For example
 * {@link com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.NowFrozenTask} defers to this once it rolls
 * out the orbit configuration changes and consensus node changesets.
 *
 * <p>A CR with mode = NetworkCheck can also be used to check the network state explicitly.
 */
@ApplicationScoped
public class NetworkCheckProcessor
        implements Processor<NetworkOperation, NetworkOperationStatus, ProcessorResult<NetworkOperationStatus>> {

    private final NetworkCheckTask networkGenesisTask;

    /**
     * Constructor for NetworkCheckProcessor.
     *
     * @param networkGenesisTask The {@link NetworkCheckTask} to be used for processing network operations.
     */
    @Inject
    public NetworkCheckProcessor(NetworkCheckTask networkGenesisTask) {
        this.networkGenesisTask = networkGenesisTask;
    }

    @Override
    public ProcessorResult<NetworkOperationStatus> process(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {
        return networkGenesisTask.execute(resource, context);
    }
}
