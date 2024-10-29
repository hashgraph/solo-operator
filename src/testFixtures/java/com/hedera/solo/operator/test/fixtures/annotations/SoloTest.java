// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.annotations;

import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContextParameterResolver;
import java.lang.annotation.ElementType;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith({SnapshotFailureContextParameterResolver.class})
@Retention(RetentionPolicy.RUNTIME)
@Target({ElementType.TYPE})
public @interface SoloTest {}
