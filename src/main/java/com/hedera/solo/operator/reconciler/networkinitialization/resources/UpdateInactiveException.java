package com.hedera.solo.operator.reconciler.networkinitialization.resources;

public class UpdateInactiveException extends RuntimeException {
    public UpdateInactiveException(String message) {
        super(message);
    }
}
