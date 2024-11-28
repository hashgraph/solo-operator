// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.utils;

import io.javaoperatorsdk.operator.ReconcilerUtils;

public class TemplateLoader {
    public static final String TEMPLATE_RECORD_STREAM_UPLOADER =
            "/com/hedera/solo/operator/test/capsule/containers/record-stream-uploader.yaml";
    public static final String TEMPLATE_EVENTS_STREAM_UPLOADER =
            "/com/hedera/solo/operator/test/capsule/containers/events-stream-uploader.yaml";
    public static final String TEMPLATE_BACKUP_UPLOADER =
            "/com/hedera/solo/operator/test/capsule/containers/backup-uploader.yaml";
    public static final String TEMPLATE_RECORD_STREAM_SIDECAR_UPLOADER =
            "/com/hedera/solo/operator/test/capsule/containers/record-stream-sidecar-files-uploader.yaml";
    public static final String TEMPLATE_OPEN_TELEMETRY_COLLECTOR =
            "/com/hedera/solo/operator/test/capsule/containers/open-telemetry-collector.yaml";

    public static <T> T load(Class<T> clazz, String templateName) {
        return ReconcilerUtils.loadYaml(clazz, TemplateLoader.class, templateName);
    }
}
