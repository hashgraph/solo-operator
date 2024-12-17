// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.utils;

public class TestRecords {
    public record NodeInfo(
            Long id, // numeric node id
            String name, // node alias e.g. nodeX where X is nodeId
            String accountId) {}
}
