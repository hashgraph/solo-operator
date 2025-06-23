// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.crdpostprocessor;

import java.nio.file.Path;
import org.eclipse.microprofile.config.ConfigProvider;

public class SoloCrdPostProcessor extends SharedCrdPostProcessor {
    public SoloCrdPostProcessor() {
        super(Path.of(
                ConfigProvider.getConfig()
                        .getOptionalValue("github.workspace", String.class)
                        .orElseGet(() -> System.getProperty("user.dir")),
                "build/generated/object-model-generator/main/crd"));
    }
}
