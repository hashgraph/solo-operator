// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.core.registry;

public class ImageRegistryException extends RuntimeException {

    public ImageRegistryException(String message) {
        super(message);
    }

    public ImageRegistryException(String message, Throwable cause) {
        super(message, cause);
    }
}
