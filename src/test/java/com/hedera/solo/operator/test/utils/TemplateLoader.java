// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.utils;

import io.javaoperatorsdk.operator.ReconcilerUtils;

/** The `TemplateLoader` class is responsible for loading predefined YAML templates. */
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
    public static final String TEMPLATE_NETWORK_UPGRADE = "/templates/network-operation/network-upgrade.yaml";

    /**
     * Loads a YAML template from the specified path and returns it as an object of the given class type.
     *
     * @param clazz The class type to which the loaded YAML should be cast.
     * @param templateName The relative path to the YAML template file.
     * @param <T> The type of the class to which the YAML content should be converted.
     * @return An instance of the specified class type representing the YAML content
     */
    public static <T> T load(Class<T> clazz, String templateName) {
        return ReconcilerUtils.loadYaml(clazz, TemplateLoader.class, templateName);
    }
}
