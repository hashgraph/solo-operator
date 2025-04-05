// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator;

import io.javaoperatorsdk.operator.Operator;
import io.quarkus.runtime.Quarkus;
import io.quarkus.runtime.QuarkusApplication;
import io.quarkus.runtime.annotations.QuarkusMain;
import jakarta.inject.Inject;
import java.time.Duration;
import java.time.temporal.ChronoUnit;

/** The {@code SoloOperator} class is the main {@code Quarkus} entrypoint for the Hedera Solo Operator. */
@QuarkusMain
public class SoloOperator implements QuarkusApplication {

    /** An injected {@link Operator} instance automatically bound to the controllers. */
    private final Operator operator;

    /**
     * Creates a new {@code SoloOperator} instance with the given {@code Operator}.
     *
     * @param operator the injected {@code Operator} instance.
     */
    @Inject
    public SoloOperator(final Operator operator) {
        this.operator = operator;
    }

    /**
     * Custom Quarkus entrypoint which starts the {@link Operator} and waits for the application to exit.
     *
     * @param args the command line arguments.
     */
    @Override
    public int run(final String... args) {
        operator.installShutdownHook(Duration.of(30, ChronoUnit.SECONDS));
        operator.start();
        Quarkus.waitForExit();
        return 0;
    }

    /**
     * The standard java main method which starts the Quarkus application.
     *
     * @param args the command line arguments.
     */
    public static void main(final String... args) {
        Quarkus.run(SoloOperator.class, args);
    }
}
