// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.hederaclient;

/** Interface for creating Hedera SDK client context factories. */
public interface HederaSdkClientContextFactory {
    /**
     * Creates a new Hedera SDK client context using the provided factory, node name, and security mode.
     *
     * @param factory The Hedera SDK client factory to use.
     * @param nodeName The name of the Hedera network node.
     * @param secure True if the connection should be secure (TLS), false otherwise.
     * @return A new Hedera SDK client context
     */
    HederaSdkClientFactory.ClientContext create(
            final HederaSdkClientFactory factory, final String nodeName, final boolean secure);
}
