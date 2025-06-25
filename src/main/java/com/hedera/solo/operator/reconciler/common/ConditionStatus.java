// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.common;

/** Enum to represent the status of a condition. */
public class ConditionStatus {
    /** Private constructor to prevent instantiation of this class. */
    private ConditionStatus() {}

    public static final String True = "True";
    public static final String False = "False";
    public static final String Unknown = "Unknown";
}
