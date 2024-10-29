// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus;

public class ConsensusCapsuleConstants {
    public static final String LABEL_SELECTOR_CONSENSUS_NODE_KEY = "app";
    public static final String LABEL_SELECTOR_CONSENSUS_NODE_VALUE = "consensus-node";
    public static final String LABEL_SELECTOR_CONSENSUS_NODE =
            LABEL_SELECTOR_CONSENSUS_NODE_KEY + "=" + LABEL_SELECTOR_CONSENSUS_NODE_VALUE;
    public static final String LABEL_CONSENSUS_CAPSULE_NAME_KEY = "consensus-capsule";
    public static final String LABEL_CONSENSUS_CAPSULE_KEY_TYPE = "key-type";

    private ConsensusCapsuleConstants() {}
}
