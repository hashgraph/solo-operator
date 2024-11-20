// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.hederaclient;

public interface HederaSdkClientContextFactory {
    HederaSdkClientFactory.ClientContext create(
            final HederaSdkClientFactory factory, final String nodeName, final boolean secure);
}
