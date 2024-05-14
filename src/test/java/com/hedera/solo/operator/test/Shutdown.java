package com.hedera.solo.operator.test;

import io.javaoperatorsdk.operator.Operator;
import io.quarkus.runtime.ShutdownEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;

@ApplicationScoped
public class Shutdown {
    @Inject
    protected Operator operator;

    void onShutdown(@Observes ShutdownEvent event) {
        operator.stop();
    }
}
