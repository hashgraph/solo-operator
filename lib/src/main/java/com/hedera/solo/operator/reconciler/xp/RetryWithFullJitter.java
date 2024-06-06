package com.hedera.solo.operator.reconciler.xp;

import io.javaoperatorsdk.operator.processing.retry.RetryExecution;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Optional;
import java.util.Random;
import java.util.concurrent.atomic.AtomicInteger;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

// full jitter:
// sleep = random_between(0, min(cap, base * 2 ** attempt))

public class RetryWithFullJitter implements RetryExecution {
    static AtomicInteger counter = new AtomicInteger();
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    int attemptCount = 0;
    final int maxAttempts;
    final long initialInterval;
    final long maxInterval;
    final Random random = new Random();

    public RetryWithFullJitter(FullJitterRetry annotation) {
        this.maxAttempts = annotation.maxAttempts();
        this.initialInterval = annotation.initialInterval();
        this.maxInterval = annotation.maxInterval();

    }

    @Override
    public Optional<Long> nextDelay() {
        if (finiteAttempts() && attemptCount >= maxAttempts) {
            return Optional.empty();
        }
        attemptCount++;
        long nextDelay = random.nextLong(Math.min(maxInterval, initialInterval * (1L << Math.min(62, attemptCount))));
        log.info("Next delay of {} ms {}", nextDelay, counter.getAndIncrement());
        return Optional.of(nextDelay);
    }

    @Override
    public int getAttemptCount() {
        return attemptCount;
    }

    @Override
    public boolean isLastAttempt() {
        return finiteAttempts() && attemptCount == maxAttempts;
    }

    boolean finiteAttempts() {
        return -1 < maxAttempts;
    }
}
