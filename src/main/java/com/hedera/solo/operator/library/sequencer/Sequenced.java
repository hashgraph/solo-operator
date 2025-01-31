// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Instant;

public record Sequenced(String namespace, String uid, String name, Instant creationTime, Instant queuedAt)
        implements Comparable<Sequenced> {

    /** Create a deep copy with a new queuedAt time */
    public static Sequenced from(Sequenced item) {
        return new Sequenced(item.namespace(), item.uid(), item.name(), item.creationTime(), Instant.now());
    }

    public static Sequenced from(HasMetadata item) {
        return new Sequenced(
                item.getMetadata().getNamespace(),
                item.getMetadata().getUid(),
                item.getMetadata().getName(),
                Instant.parse(item.getMetadata().getCreationTimestamp()),
                Instant.now());
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
