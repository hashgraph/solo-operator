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
 * @param <T> custom resource type
 * @param <S> custom resource status type
 */
public final class RetryStep<T extends CustomResource<?, S>, S> implements ProcessorStep<T, S> {
    private final ProcessorStep<T, S> step;
    private final Duration retryInterval;
    private final int maxRetries;
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    /**
     * Create a new RetryStep with defaults
     *
     * @param step the step to retry
     */
    public RetryStep(final ProcessorStep<T, S> step) {
        this(step, Duration.ofSeconds(2), 30); // default retry for 1 minutes
    }

    /**
     * Create a new RetryStep with the specified retry interval and maximum retries.
     *
     * @param step the step to retry
     * @param retryInterval the interval between retries
     * @param maxRetries the maximum number of retries
     */
    public RetryStep(final ProcessorStep<T, S> step, final Duration retryInterval, final int maxRetries) {
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
    public S run(final T resource, final S status, final Context<T> context) {
        S updated = step.run(resource, status, context);

        int retries = 0;
        while (!step.isSuccessful(resource, updated) && retries++ < maxRetries) {
            log.warnf(
                    "'%s' failed. Retrying in %s seconds (%s/%s)... [%d/%d]",
                    step.getClass().getSimpleName(),
                    retryInterval.getSeconds(),
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    retries,
                    maxRetries);

            this.applyRetryInterval();

            // Note: we pass the updated status to the next run so that a step can store info in status object for
            // subsequent runs
            updated = step.run(resource, updated, context);
        }

        return updated;
    }

    @Override
    public boolean isSuccessful(final T resource, final S status) {
        return step.isSuccessful(resource, status);
    }

    @Override
    public boolean shouldSkip(final T resource, final S status) {
        return step.shouldSkip(resource, status);
    }

    private void applyRetryInterval() {
        try {
            Thread.sleep(retryInterval.toMillis());
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            log.error(new ProcessorException("Interrupted while waiting for retry interval", e));
        }
    }
}
