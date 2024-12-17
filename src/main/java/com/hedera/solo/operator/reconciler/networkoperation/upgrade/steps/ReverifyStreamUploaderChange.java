// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.AbstractStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

/**
 * Revalidate the selected image for the stream uploader image still exists, if not, choose the next available image. If
 * no images are available, fail the operation.
 */
public class ReverifyStreamUploaderChange extends AbstractStep<NetworkOperation> {
    private final String containerName;

    public ReverifyStreamUploaderChange(String name) {
        this.containerName = name;
    }

    @Override
    public NetworkOperation execute(NetworkOperation resource, Context<NetworkOperation> context) {
        final StreamUploaderChangeset changeset =
                VerifyStreamUploaderChange.getStreamUploaderChangeset(resource, containerName);

        if (Objects.isNull(changeset)) {
            return resource; // no-op
        }

        throw new UnsupportedOperationException("Not implemented");
    }
}
