// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.core.registry;

/** Exception thrown by {@link ImageRegistry} when there is an error fetching or processing image manifests. */
public class ImageRegistryException extends RuntimeException {

    /**
     * Constructs a new {@code ImageRegistryException} with the specified detail message.
     *
     * @param message The detail message (which is saved for later retrieval by the {@link #getMessage()} method).
     */
    public ImageRegistryException(String message) {
        super(message);
    }

    /**
     * Constructs a new {@code ImageRegistryException} with the specified detail message and cause.
     *
     * @param message The detail message (which is saved for later retrieval by the {@link #getMessage()} method).
     * @param cause The cause (which is saved for later retrieval by the {@link #getCause()} method).
     */
    public ImageRegistryException(String message, Throwable cause) {
        super(message, cause);
    }
}
