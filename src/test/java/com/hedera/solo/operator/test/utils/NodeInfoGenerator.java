// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.utils;

import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import java.util.ArrayList;
import java.util.List;

/** Utility class for generating node information used in tests. */
public class NodeInfoGenerator {
    /**
     * Generate account ID for a given node ID.
     *
     * @param nodeId The node ID for which to generate the account ID.
     * @return The generated account ID as a string
     */
    public static String accountId(Long nodeId) {
        // FIXME: This is incorrect, but currently platform code assumes that account ID = node ID + 3
        return String.format("0.0.%d", nodeId + Constants.HEDERA_ACCOUNT_START);
    }

    /**
     * Generate NodeInfo for a given node ID and namespace.
     *
     * @param nodeId The node ID for which to generate the NodeInfo.
     * @param namespace The namespace in which the node resides.
     * @return A TestRecords.NodeInfo object containing the node information
     */
    public static TestRecords.NodeInfo nodeInfo(Long nodeId, String namespace) {
        final String nodeName = Metadata.prepareNodeName(nodeId);
        return new TestRecords.NodeInfo(nodeId, nodeName, accountId(nodeId));
    }

    /**
     * Generate a list of NodeInfo objects for a given list of node IDs and namespace.
     *
     * @param nodeIds The list of node IDs for which to generate the NodeInfo objects.
     * @param namespace The namespace in which the nodes reside.
     * @return A list of TestRecords.NodeInfo objects containing the node information
     */
    public static List<TestRecords.NodeInfo> nodes(List<Long> nodeIds, String namespace) {
        List<TestRecords.NodeInfo> nodes = new ArrayList<>();
        for (Long nodeId : nodeIds) {
            nodes.add(nodeInfo(nodeId, namespace));
        }

        return nodes;
    }

    /**
     * Generate a list of node IDs starting from 0 up to the specified count.
     *
     * @param count The number of node IDs to generate.
     * @return A list of long representing the generated node IDs
     */
    public static List<Long> nodeIds(int count) {
        long nodeId = 0L;
        List<Long> nodeIds = new ArrayList<>();
        while (count-- > 0) {
            nodeIds.add(nodeId++);
        }

        return nodeIds;
    }
}
