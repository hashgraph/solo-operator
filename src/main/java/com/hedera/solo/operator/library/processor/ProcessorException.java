// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

public class ProcessorException extends RuntimeException {
    public static final String ERR_MSG_PROCESSOR_FAILED = "Processor failed";

    public ProcessorException(String message) {
        super(message);
    }

    public ProcessorException(String message, Throwable cause) {
        super(message, cause);
    }

    public ProcessorException(Throwable cause) {
        super(ERR_MSG_PROCESSOR_FAILED, cause);
    }
}
