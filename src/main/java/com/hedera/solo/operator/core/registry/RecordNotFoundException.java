// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.core.registry;

/** Custom exception thrown when a record is not found in the registry. */
public class RecordNotFoundException extends RuntimeException {

    /**
     * Constructs a new {@code RecordNotFoundException} with the specified detail message.
     *
     * @param message the detail message (which is saved as a throwable object)
     */
    public RecordNotFoundException(String message) {
        super(message);
    }

    /**
     * Constructs a new {@code RecordNotFoundException} with the specified detail message and cause.
     *
     * @param message the detail message (which is saved as a throwable object)
     * @param cause the cause (which is saved as a throwable object)
     */
    public RecordNotFoundException(String message, Throwable cause) {
        super(message, cause);
    }
}
