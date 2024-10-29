// SPDX-License-Identifier: Apache-2.0

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
            log.error(e.getStdErr());
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
            log.error(e.getStdErr());
            throw e;
        }
        setupDone = false;
    }
}
