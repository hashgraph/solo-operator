// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.fuzz;

import io.fabric8.kubernetes.api.model.Quantity;

/**
 * The {@code KubernetesResourceFactory} class is a specialized factory for creating instances of Kubernetes resources,
 * particularly the Quantity type. It extends the generic BeanFactory to provide specific implementations and custom
 * behavior for generating Quantity objects.
 */
public class KubernetesResourceFactory extends BeanFactory {
    /** Constructs a new {@code KubernetesResourceFactory} with no seed value. */
    public KubernetesResourceFactory() {
        this(null);
    }

    /**
     * Constructs a new {@code KubernetesResourceFactory} with the provided seed value for randomization purposes.
     *
     * @param seed A Long value used as a seed for generating random numbers, useful for test reproducibility.
     */
    public KubernetesResourceFactory(final Long seed) {
        super(seed);
        supplier(Quantity.class, (f, t) -> new Quantity(f.nonNull(Integer.class).toString()));
    }
}
