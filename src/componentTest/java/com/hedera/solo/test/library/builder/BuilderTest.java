// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.library.builder;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import org.junit.jupiter.api.Test;

/** The BuilderTest class is a JUnit test case for testing the functionality of the {@link BuilderWrapper} class. */
public class BuilderTest {
    /**
     * A custom exception class for testing purposes. This exception is checked, meaning it must be declared in the
     * method signature or caught.
     */
    static class TestCheckedException extends Exception {}

    /**
     * A custom unchecked exception class for testing purposes. This exception is a runtime exception, meaning it does
     * not need to be declared in the method signature.
     */
    static class TestUncheckedException extends RuntimeException {}

    /**
     * A method to add a name to a ConfigMapBuilder.
     *
     * @param name the name to be added to the ConfigMapBuilder.
     * @return a BuilderFunction that adds the specified name to the builder
     */
    BuilderFunction<ConfigMapBuilder> name(final String name) {
        return builder -> builder.editMetadata().withName(name);
    }

    /**
     * A method to throw an exception from a ConfigMapBuilder.
     *
     * @param e the exception to be thrown.
     * @return a BuilderFunction that throws the specified exception
     * @throws Exception if the exception is not of type RuntimeException, it will be wrapped in a RuntimeException.
     */
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
