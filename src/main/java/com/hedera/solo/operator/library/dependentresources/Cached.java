package com.hedera.solo.operator.library.dependentresources;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.stream.Stream;

public class Cached {
    public static <N extends HasMetadata, C extends HasMetadata> Stream<C> primaries(N namespaced, Context<C> context) {
        String namespace = namespaced.getMetadata().getNamespace();
        return context.getPrimaryCache().list().filter(p -> namespace.equals(p.getMetadata().getNamespace()));
    }
}
