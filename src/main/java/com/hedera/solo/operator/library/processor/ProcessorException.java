// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

/**
 * Exception thrown by the processor. Every exception thrown by the processor, task or step should be an instance of
 * this class.
 */
public class ProcessorException extends RuntimeException {
    /**
     * Constructs a new ProcessorException with the specified cause.
     *
     * @param message the detail message
     */
    public ProcessorException(final String message) {
        super(message);
    }

    /**
     * Constructs a new ProcessorException with the specified detail message and cause.
     *
     * @param message the detail message
     * @param cause the cause of the exception
     */
    public ProcessorException(final String message, final Throwable cause) {
        super(message, cause);
    }
}
