// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

public interface SequenceResourceManager<
        T extends CustomResource<S, R>, S extends KubernetesResource, R extends KubernetesResource> {
    /**
     * Get the name of the resource manager
     *
     * <p>It is used as the suffix of the config map name used by the {@link Sequencer} to store the sequence numbers
     * for resources managed by this resource manager
     *
     * @return the name of the resource
     */
    String getName();

    /**
     * Get the orbit name from the spec field of the resource
     *
     * @param resource the resource to get the orbit name
     * @return the orbit name
     */
    String getOrbitName(T resource);

    /**
     * Get the sequence number from the status field of the resource
     *
     * @param resource the resource to get the sequence number
     * @return the sequence number
     */
    Long getSequenceNumber(T resource);

    /**
     * Set the sequence number in the status field of the resource and add additional metadata if required
     *
     * @param resource the resource to set the sequence number
     * @param context the context object
     * @param sequenceNumber the sequence number to set
     * @return the resource with the sequence number and additional metadata set
     */
    T setSequenceNumber(T resource, Context<T> context, Long sequenceNumber);
}
