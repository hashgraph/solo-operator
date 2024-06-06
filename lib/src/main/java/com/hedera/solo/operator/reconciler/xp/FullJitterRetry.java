package com.hedera.solo.operator.reconciler.xp;

import io.javaoperatorsdk.operator.processing.retry.GradualRetry;
import java.lang.annotation.ElementType;
import java.lang.annotation.Inherited;
import java.lang.annotation.Retention;
import java.lang.annotation.RetentionPolicy;
import java.lang.annotation.Target;
import java.time.Duration;

@Inherited
@Retention(RetentionPolicy.RUNTIME)
@Target(ElementType.TYPE)
public @interface FullJitterRetry {
    int maxAttempts() default 5;

    long initialInterval() default 1_000;

    long maxInterval() default 10_000;
}
