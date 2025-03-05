// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

/** Exception thrown by the sequencer. Every exception thrown by the sequencer should be an instance of this class. */
public class SequencerException extends RuntimeException {
    public SequencerException(final String message) {
        super(message);
    }

    public SequencerException(final String message, final Throwable cause) {
        super(message, cause);
    }
}
