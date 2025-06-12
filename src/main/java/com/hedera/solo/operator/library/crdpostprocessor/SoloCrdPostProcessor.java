// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.crdpostprocessor;

import com.hedera.solo.operator.buildlogic.sharedlib.SharedCrdPostProcessor;
import java.nio.file.Path;
import org.eclipse.microprofile.config.ConfigProvider;

public class SoloCrdPostProcessor extends SharedCrdPostProcessor {
    public SoloCrdPostProcessor() {
        super(Path.of(
                ConfigProvider.getConfig()
                        .getOptionalValue("quarkus.project.root.dir", String.class)
                        .orElse(""),
                "build/generated/object-model-generator/main/crd"));
    }
}
