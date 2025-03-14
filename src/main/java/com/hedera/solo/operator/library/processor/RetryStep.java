// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * A wrapper class to execute a step multiple times with a delay between each attempt.
 *
 * <p>The step needs to throw an exception if it fails in order to trigger a retry.
 *
 * <p>It will retry the step until it succeeds or the maximum number of retries is reached.
 *
 * <p>After the maximum number of retries is reached, the step will call {@link ProcessorStep#handleError} to handle the
 * last exception.
 *
 * @param <T> custom resource type
 * @param <S> custom resource status type
 */
public final class RetryStep<T extends CustomResource<?, S>, S> implements ProcessorStep<T, S> {
    private final ProcessorStep<T, S> step;
    private final Duration retryInterval;
    private final int maxRetries;
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    /**
     * Create a new RetryStep with default retry interval of 5 seconds and maximum retries of 12.
     *
     * @param step the step to retry
     */
    public RetryStep(ProcessorStep<T, S> step) {
        this(step, Duration.ofSeconds(5), 12); // default retry for 1 minutes
    }

    /**
     * Create a new RetryStep with the specified retry interval and maximum retries.
     *
     * @param step the step to retry
     * @param retryInterval the interval between retries
     * @param maxRetries the maximum number of retries
     */
    public RetryStep(ProcessorStep<T, S> step, Duration retryInterval, int maxRetries) {
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

                this.applyRetryInterval();
                retries++;
            }
        }

        throw new ProcessorException("Failed to execute step after maximum retries", lastException);
    }

    private void applyRetryInterval() {
        try {
            Thread.sleep(retryInterval.toMillis());
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new ProcessorException("Interrupted while waiting for retry interval", e);
        }
    }

    @Override
    public S handleError(T resource, S status, Context<T> context, Exception ex) {
        return step.handleError(resource, status, context, ex);
    }
}
