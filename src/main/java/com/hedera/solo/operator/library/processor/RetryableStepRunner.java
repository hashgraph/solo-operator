// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * A wrapper class to execute a RetryableStep multiple times with a delay between each attempt.
 *
 * <p>Any existing step can be converted to a retryable step by implementing {@link RetryableStep} interface and
 * wrapping it with this class.
 *
 * <p>If the step fails, it will log the error and retry the step after the specified delay.
 *
 * <p>It will retry the step until it succeeds or the maximum number of retries is reached.
 *
 * <p>After the maximum number of retries is reached, the step will call {@link RetryableStep#handleRetryError} to
 * handle the last exception.
 *
 * @param <T> custom resource type
 * @param <S> custom resource status type
 */
public final class RetryableStepRunner<T extends CustomResource<?, S>, S> implements ProcessorStep<T, S> {
    private final RetryableStep<T, S> step;
    private final Duration retryInterval;
    private final int maxRetries;
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    public RetryableStepRunner(RetryableStep<T, S> step, Duration retryInterval, int maxRetries) {
        Objects.requireNonNull(step, "retryable step must not be null");
        Objects.requireNonNull(retryInterval, "retryInterval must not be null");

        if (retryInterval.isNegative() || retryInterval.isZero()) {
            throw new IllegalArgumentException("retryInterval must be greater than 0");
        }

        if (maxRetries <= 0) {
            throw new IllegalArgumentException("maxRetries must be greater than 0");
        }

        this.step = step;
        this.retryInterval = retryInterval;
        this.maxRetries = maxRetries;
    }

    @Override
    public S run(T resource, S status, Context<T> context) {
        Exception lastException = null;
        int retries = 1;
        while (retries <= maxRetries) {
            try {
                return step.run(resource, status, context);
            } catch (Exception e) {
                lastException = e;
                log.warnf(
                        "%s. Retrying in %s seconds... [%d/%d]",
                        e.getMessage(), retryInterval.getSeconds(), retries, maxRetries);

                try {
                    Thread.sleep(retryInterval.toMillis());
                } catch (InterruptedException ex) {
                    Thread.currentThread().interrupt();
                    log.errorf("Interrupted while waiting for retry: %s", ex.getMessage());
                    break;
                }

                retries++;
            }
        }

        return step.handleRetryError(resource, status, context, lastException);
    }
}
