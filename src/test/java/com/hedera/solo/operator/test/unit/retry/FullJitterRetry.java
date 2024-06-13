package com.hedera.solo.operator.test.unit.retry;

import java.lang.annotation.ElementType;
import java.lang.annotation.Inherited;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;

@Inherited
@Retention(RetentionPolicy.RUNTIME)
@Target(ElementType.TYPE)
public @interface FullJitterRetry {
    int maxAttempts() default 5;

    long initialInterval() default 1_000;

    long maxInterval() default 10_000;
}
