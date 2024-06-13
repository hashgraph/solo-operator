package com.hedera.solo.operator.test.unit;

import io.javaoperatorsdk.operator.api.config.AnnotationConfigurable;
import io.javaoperatorsdk.operator.processing.retry.Retry;
import io.javaoperatorsdk.operator.processing.retry.RetryExecution;

public class MyRetry implements Retry, AnnotationConfigurable<FullJitterRetry> {
    FullJitterRetry annotation;
    @Override
    public void initFrom(FullJitterRetry fullJitterRetry) {
        annotation = fullJitterRetry;
    }

    @Override
    public RetryExecution initExecution() {
        return new RetryWithFullJitter(annotation);
    }
}
