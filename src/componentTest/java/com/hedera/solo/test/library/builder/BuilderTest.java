// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.library.builder;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import org.junit.jupiter.api.Test;

public class BuilderTest {
    static class TestCheckedException extends Exception {}

    static class TestUncheckedException extends RuntimeException {}

    BuilderFunction<ConfigMapBuilder> name(final String name) {
        return builder -> builder.editMetadata().withName(name);
    }

    BuilderFunction<ConfigMapBuilder> testThrow(final Exception e) throws Exception {
        return builder -> {
            throw e;
        };
    }

    @Test
    public void test() throws Exception {
        RuntimeException e = assertThrows(RuntimeException.class, () -> {
            edit(new ConfigMap())
                    .with(name("name1"))
                    .with(testThrow(new TestCheckedException()))
                    .build();
        });
        assertTrue(e.getCause() instanceof TestCheckedException);
        assertThrows(TestUncheckedException.class, () -> {
            edit(new ConfigMap())
                    .with(name("name2"))
                    .with(testThrow(new TestUncheckedException()))
                    .build();
        });
    }
}
