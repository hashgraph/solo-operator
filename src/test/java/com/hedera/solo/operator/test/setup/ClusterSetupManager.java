// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.setup;

import com.hedera.fullstack.helm.client.HelmExecutionException;
import io.quarkus.runtime.ShutdownEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import javax.annotation.Priority;
import org.jboss.logging.Logger;

//@ApplicationScoped
public class ClusterSetupManager {
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private static boolean setupDone = false;

    @Inject
    MinioSetupManager minioSetupManager;

    /** Check and installed required services */
    public synchronized void check() {
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

    /**
     * Cleanup upon Shutdown event
     *
     * @param event Shutdown event
     */
    public synchronized void cleanup(@Observes @Priority(1) ShutdownEvent event) {
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
