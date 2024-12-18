// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container;

import com.hedera.solo.operator.library.container.oci.OCIManifest;
import java.net.http.HttpResponse;

/** Manifest response from registry including unmarshalled manifest for recognized media types. */
public class ManifestResponse {
    public final HttpResponse<byte[]> response;
    public final OCIManifest manifest;

    public ManifestResponse(final HttpResponse<byte[]> response) {
        this.response = response;
        manifest =
                OCIManifest.from(response.headers().firstValue("Content-Type").orElse(null), response.body());
    }
}
