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
