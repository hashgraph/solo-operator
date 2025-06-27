// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.utils;

/** Represents information about a test node including its ID, name, and account ID. */
public class TestRecords {
    /**
     * A record that holds information about a node in the test environment.
     *
     * @param id The numeric ID of the node.
     * @param name The alias of the node, typically formatted as "nodeX" where X is the node ID.
     * @param accountId The account ID associated with the node.
     */
    public record NodeInfo(
            Long id, // numeric node id
            String name, // node alias e.g. nodeX where X is nodeId
            String accountId) {}
}
