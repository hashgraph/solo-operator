// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.core.registry;

public class RecordNotFoundException extends RuntimeException {

    public RecordNotFoundException(String message) {
        super(message);
    }

    public RecordNotFoundException(String message, Throwable cause) {
        super(message, cause);
    }
}
