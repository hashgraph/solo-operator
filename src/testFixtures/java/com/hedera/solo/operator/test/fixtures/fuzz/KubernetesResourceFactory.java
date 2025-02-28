// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.fuzz;

import io.fabric8.kubernetes.api.model.Quantity;

public class KubernetesResourceFactory extends BeanFactory {
    public KubernetesResourceFactory() {
        this(null);
    }

    public KubernetesResourceFactory(final Long seed) {
        super(seed);
        supplier(Quantity.class, (f, t) -> new Quantity(f.nonNull(Integer.class).toString()));
    }
}
