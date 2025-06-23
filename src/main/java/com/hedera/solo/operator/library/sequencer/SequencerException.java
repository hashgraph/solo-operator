// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

/** Exception thrown by the sequencer. Every exception thrown by the sequencer should be an instance of this class. */
public class SequencerException extends RuntimeException {
    /**
     * Constructs a new SequencerException with the specified cause.
     *
     * @param message the detail message
     */
    public SequencerException(final String message) {
        super(message);
    }

    /**
     * Constructs a new SequencerException with the specified detail message and cause.
     *
     * @param message the detail message
     * @param cause the cause of the exception
     */
    public SequencerException(final String message, final Throwable cause) {
        super(message, cause);
    }
}
