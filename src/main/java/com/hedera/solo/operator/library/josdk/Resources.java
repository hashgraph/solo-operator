// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.josdk;

import static com.hedera.solo.operator.library.generic.Iterables.asStream;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;

public class Resources {
    private Resources() {}
    /** Convert resources to map with string identifiers. */
    public static <R extends HasMetadata> Map<String, R> toMap(final Iterable<R> iterable) {
        return asStream(iterable).collect(Collectors.toMap(ResourceID::toString, Function.identity()));
    }
}
