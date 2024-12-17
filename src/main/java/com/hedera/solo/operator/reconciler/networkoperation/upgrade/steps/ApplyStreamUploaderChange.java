// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.AbstractStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

public class ApplyStreamUploaderChange extends AbstractStep<NetworkOperation> {
    private final String containerName;

    public ApplyStreamUploaderChange(String name) {
        this.containerName = name;
    }

    @Override
    public NetworkOperation execute(NetworkOperation resource, Context<NetworkOperation> context) {
        final StreamUploaderChangeset changeset =
                VerifyStreamUploaderChange.getStreamUploaderChangeset(resource, this.containerName);

        if (Objects.isNull(changeset)) {
            return resource; // no-op
        }

        throw new UnsupportedOperationException("Not implemented");
    }
}
