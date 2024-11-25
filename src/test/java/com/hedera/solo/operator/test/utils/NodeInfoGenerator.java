// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.utils;

import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import java.util.ArrayList;
import java.util.List;

public class NodeInfoGenerator {
    /** Generate account ID for a given node ID */
    public static String accountId(Long nodeId) {
        // FIXME: This is incorrect, but currently platform code assumes that account ID = node ID + 3
        return String.format("0.0.%d", nodeId + Constants.HEDERA_ACCOUNT_START);
    }

    public static String internalK8sServiceName(String nodeName, String namespace) {
        return String.format("%s.consensus.%s.svc.cluster.local", nodeName, namespace);
    }

    public static String externalK8sServiceName(String nodeName, String namespace) {
        return String.format("%s-svc.%s.svc.cluster.local", nodeName, namespace);
    }

    public static TestRecords.NodeInfo nodeInfo(Long nodeId, String namespace) {
        final String nodeName = Metadata.prepareNodeName(nodeId);
        return new TestRecords.NodeInfo(
                nodeId,
                nodeName,
                accountId(nodeId),
                1, // staking weight
                internalK8sServiceName(nodeName, namespace),
                externalK8sServiceName(nodeName, namespace));
    }

    public static List<TestRecords.NodeInfo> nodes(List<Long> nodeIds, String namespace) {
        List<TestRecords.NodeInfo> nodes = new ArrayList<>();
        for (Long nodeId : nodeIds) {
            nodes.add(nodeInfo(nodeId, namespace));
        }

        return nodes;
    }

    public static List<Long> nodeIds(int count) {
        long nodeId = 0L;
        List<Long> nodeIds = new ArrayList<>();
        while (count-- > 0) {
            nodeIds.add(nodeId++);
        }

        return nodeIds;
    }
}
