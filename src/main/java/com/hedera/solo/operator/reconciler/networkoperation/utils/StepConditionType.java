// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

/**
 * Represents a condition type for network operation steps, containing a type string and a reason string.
 *
 * @param type The type of the condition.
 * @param reason The reason for the condition.
 */
public record StepConditionType(String type, String reason) {}
