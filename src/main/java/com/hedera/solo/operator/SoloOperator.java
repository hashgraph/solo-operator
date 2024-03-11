/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
