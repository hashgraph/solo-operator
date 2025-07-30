// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import io.quarkus.test.junit.QuarkusTest;
import org.junit.jupiter.api.Tag;

/**
 * The {@code ConsensusCapsuleTest} class is a test class for the consensus capsule functionality. It extends
 * {@link ConsensusCapsuleTestAbstract}, which contains common setup, teardown, and test methods.
 */
@QuarkusTest
@Tag("helm-test")
public class ConsensusCapsuleTest extends ConsensusCapsuleTestAbstract {}
