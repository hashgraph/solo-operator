// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy;

public class HAProxyCapsuleConstants {
    public static final String LABEL_SELECTOR_HAPROXY_KEY = "app";
    public static final String LABEL_SELECTOR_HAPROXY_VALUE = "haproxy";
    public static final String LABEL_SELECTOR_HAPROXY = LABEL_SELECTOR_HAPROXY_KEY + "=" + LABEL_SELECTOR_HAPROXY_VALUE;
    public static final String LABEL_HAPROXY_NAME_KEY = "haproxy";

    private HAProxyCapsuleConstants() {}
}
