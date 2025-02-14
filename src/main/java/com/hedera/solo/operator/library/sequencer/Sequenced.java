// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Instant;

/**
 * Stores metadata of a resource that is stored in an ordered list in the cache
 *
 * @param namespace
 * @param uid
 * @param name
 * @param creationTime
 */
public record Sequenced(String namespace, String uid, String name, Instant creationTime)
        implements Comparable<Sequenced> {

    /** Create a record from resource */
    public static Sequenced from(HasMetadata resource) {
        return new Sequenced(
                resource.getMetadata().getNamespace(),
                resource.getMetadata().getUid(),
                resource.getMetadata().getName(),
                Instant.parse(resource.getMetadata().getCreationTimestamp()));
    }

    public static Sequenced clone(Sequenced item) {
        return new Sequenced(item.namespace, item.uid, item.name, item.creationTime);
    }

    @Override
    public int compareTo(Sequenced o) {
        if (o == null) {
            return 1;
        }

        int r = creationTime.compareTo(o.creationTime);
        if (r != 0) {
            return r;
        }

        return name.compareTo(o.name);
    }
}
