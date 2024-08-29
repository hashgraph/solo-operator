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

package com.hedera.solo.operator.test.setup;

import com.hedera.fullstack.helm.client.HelmExecutionException;
import io.quarkus.arc.Lock;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.util.concurrent.TimeUnit;
import javax.annotation.Priority;
import org.jboss.logging.Logger;

@Lock(time = 10, unit = TimeUnit.MINUTES) // this lock is needed when tests are run in parallel
@ApplicationScoped
public class ClusterSetupManager {
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private static boolean setupDone = false;

    @Inject
    MinioSetupManager minioSetupManager;

    public void setup(@Observes @Priority(1) StartupEvent event) {
        if (setupDone) {
            return;
        }

        try {
            minioSetupManager.install();
        } catch (HelmExecutionException e) {
            log.error("helm stdout: " + e.getStdOut());
            log.error("helm stderr: " + e.getStdErr());
            throw e;
        }
        setupDone = true;
    }

    public void cleanup(@Observes @Priority(1) ShutdownEvent event) {
        if (!setupDone) {
            return;
        }

        try {
            minioSetupManager.uninstall();
        } catch (HelmExecutionException e) {
            log.error("helm stdout: " + e.getStdOut());
            log.error("helm stderr: " + e.getStdErr());
            throw e;
        }
        setupDone = false;
    }
}
