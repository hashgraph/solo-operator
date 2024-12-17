// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.AbstractStep;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import io.javaoperatorsdk.operator.api.reconciler.Context;

public class ApplyFirewallChange extends AbstractStep<NetworkOperation> {
    @Override
    public NetworkOperation execute(NetworkOperation resource, Context<NetworkOperation> context) {
        // TODO: store existing firewall rules in the status

        // TODO: apply firewall changes

        throw new UnsupportedOperationException("Not implemented");
    }

    @Override
    public NetworkOperation undo(NetworkOperation resource, Context<NetworkOperation> context)
            throws ProcessorException {

        // TODO: undo firewall changes base on the stored rules in the status

        throw new UnsupportedOperationException("Not implemented");
    }
}
