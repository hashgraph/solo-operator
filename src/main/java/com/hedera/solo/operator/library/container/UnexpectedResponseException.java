// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container;

import java.net.http.HttpResponse;

/** Exception to wrap any unexpected response from OCI registry. */
public class UnexpectedResponseException extends RuntimeException {
    public final HttpResponse<byte[]> response;

    /**
     * Constructs a new UnexpectedResponseException with the specified detail message and cause.
     *
     * @param response The HTTP response received from the OCI registry.
     */
    public UnexpectedResponseException(final HttpResponse<byte[]> response) {
        super("Unexpected response code: " + response.statusCode());
        this.response = response;
    }
}
