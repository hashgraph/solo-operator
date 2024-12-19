// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

/**
 * Exception thrown by the processor. Every exception thrown by the processor, task or step should be an instance of
 * this class.
 */
public class ProcessorException extends RuntimeException {
    public ProcessorException(final String message) {
        super(message);
    }

    public ProcessorException(final String message, final Throwable cause) {
        super(message, cause);
    }
}
