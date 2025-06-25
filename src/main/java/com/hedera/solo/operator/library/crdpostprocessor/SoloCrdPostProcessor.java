// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.crdpostprocessor;

import java.nio.file.Path;
import org.eclipse.microprofile.config.ConfigProvider;

/**
 * This class is a specific implementation of {@link SharedCrdPostProcessor} tailored for solo operation, where it
 * processes Custom Resource Definitions (CRDs) after their generation. It uses the configuration to determine the base
 * directory and subdirectory for CRD files during processing.
 */
public class SoloCrdPostProcessor extends SharedCrdPostProcessor {
    /**
     * Constructs a new {@code SoloCrdPostProcessor} instance, initializing it with the default paths for CRD processing
     * based on the configuration and system properties.
     */
    public SoloCrdPostProcessor() {
        super(Path.of(
                ConfigProvider.getConfig()
                        .getOptionalValue("project.root", String.class)
                        .orElseGet(() -> System.getProperty("user.dir")),
                "build/generated/object-model-generator/main/crd"));
    }
}
