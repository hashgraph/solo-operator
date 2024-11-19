// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.common;

import com.hedera.hashgraph.sdk.AccountId;
import com.hedera.hashgraph.sdk.PrivateKey;
import io.fabric8.kubernetes.api.model.PodSecurityContextBuilder;
import java.util.Map;

public final class Constants {
    private Constants() {
        throw new UnsupportedOperationException("Utility class");
    }

    // defaults
    public static final String SOLO_OPERATOR = "solo-operator";
    public static final String SOLO_OPERATOR_VERSION = "0.0.1"; // FIXME load from pom
    public static final String CONSENSUS_NODE = "consensus-node";
    public static final Long HEDERA_USER = 2000L;
    public static final Long HEDERA_GROUP = 2000L;
    public static final PodSecurityContextBuilder SECURITY_CONTEXT_BUILDER = new PodSecurityContextBuilder()
            .withRunAsUser(HEDERA_USER)
            .withRunAsGroup(HEDERA_GROUP)
            .withFsGroup(HEDERA_GROUP)
            .withFsGroupChangePolicy("OnRootMismatch");
    public static final int DEFAULT_FILE_MODE = 493; // 0755
    public static final String NODE_NAME_FORMAT = "node%d";

    // HEDERA related
    public static final int HEDERA_ACCOUNT_START = 3;
    public static final AccountId OPERATOR_ACCOUNT_ID = AccountId.fromString("0.0.2");
    public static final PrivateKey OPERATOR_PRIVATE_KEY = PrivateKey.fromString(
            "302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137");
    public static final int GOSSIP_PORT = 50111;
    public static final int GRPC_PORT = 50211;
    public static final int GRPC_TLS_PORT = 50212;

    // Helm chart related
    public static final String MINIO_CHART_REPO = "minio-operator";
    public static final String MINIO_CHART_REPO_URL = "https://operator.min.io";
    public static final String MINIO_OPERATOR_CHART = String.format("%s/operator", MINIO_CHART_REPO);
    public static final String MINIO_TENANT_CHART = String.format("%s/tenant", MINIO_CHART_REPO);
    public static final String MINIO_OPERATOR_RELEASE = "minio-operator";
    public static final String MINIO_OPERATOR_LABEL = "app.kubernetes.io/instance";
    public static final String MINIO_TENANT_RELEASE = "minio-tenant";
    public static final String MINIO_TENANT_LABEL = "v1.min.io/tenant";
    public static final String MINIO_OPERATOR_NAMESPACE = "minio-operator";
    public static final String MINIO_TENANT_NAMESPACE = "minio-tenant";

    // template variables
    public static final String TPL_VAR_NAMESPACE = "namespace";
    public static final String TPL_VAR_DEFAULT_MODE = "defaultMode"; // e.g. 0755
    public static final String TPL_VAR_NODE_ID = "nodeId";
    public static final String TPL_VAR_NODE_IDS = "nodeIds";
    public static final String TPL_VAR_NODE_NAME = "nodeName";
    public static final String TPL_VAR_NODE_NAMES = "nodeNames";
    public static final String TPL_VAR_ACCOUNT_ID = "accountId";
    public static final String TPL_VAR_SECURITY_CONTEXT = "securityContext";
    public static final String TPL_VAR_PUBLIC_KEY_FILES = "publicKeyFiles";
    public static final String TPL_VAR_CONSENSUS_NODE = "cn";
    public static final String TPL_VAR_RECORD_STREAM_UPLOADER = "rsu";
    public static final String TPL_VAR_EVENT_STREAM_UPLOADER = "esu";
    public static final String TPL_VAR_BACKUP_UPLOADER = "bu";
    public static final String TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER = "rssu";

    // Sidecars
    public static final String RECORD_STREAM_BUCKET_PATH_TEMPLATE = "recordstreams/record%s";
    public static final String RECORD_STREAM_EXTENSION = "rcd";
    public static final String RECORD_STREAM_SIG_EXTENSION = "rcd_sig";

    public static final String EVENT_STREAM_BUCKET_PATH_TEMPLATE = "eventsStreams/events_%s";
    public static final String EVENT_STREAM_EXTENSION = "evts";
    public static final String EVENT_STREAM_SIG_EXTENSION = "evts_sig";

    // labels
    public static final String OPERATOR_PREFIX = "operator.solo.hedera.com/";
    public static final String LABEL_APP = "app";
    public static final String LABEL_ACCOUNT_ID = OPERATOR_PREFIX + "account-id"; // e.g. 0.0.3
    public static final String LABEL_NODE_ID = OPERATOR_PREFIX + "node-id"; // e.g. 0, 1, 3 etc.
    public static final String LABEL_NODE_NAME = OPERATOR_PREFIX + "node-name"; // e.g. node0
    public static final String LABEL_ORBIT = OPERATOR_PREFIX + "orbit-name";
    public static final String LABEL_KEY_TYPE = OPERATOR_PREFIX + "key-type";
    public static final String LABEL_K8S_NAME = "app.kubernetes.io/name";
    public static final String LABEL_K8S_INSTANCE = "app.kubernetes.io/instance";
    public static final String LABEL_K8S_VERSION = "app.kubernetes.io/version";
    public static final String LABEL_K8S_COMPONENT = "app.kubernetes.io/component";
    public static final String LABEL_K8S_PART_OF = "app.kubernetes.io/part-of";
    public static final String LABEL_K8S_MANAGED_BY = "app.kubernetes.io/managed-by";
    public static final String LABEL_SELECTOR_APP_CONSENSUS_NODE = Constants.LABEL_APP + "=" + Constants.CONSENSUS_NODE;
    public static final String LABEL_SELECTOR_APP_HAPROXY = Constants.LABEL_APP + "=" + Constants.COMPONENT_HAPROXY;

    // key types
    public static final String KEY_TYPE_GOSSIP = "gossip";
    public static final String KEY_TYPE_GRPC_TLS = "grpc-tls";

    // volumes
    public static final String VOLUME_LOGS_NAME_SUFFIX = "output";
    public static final String VOLUME_STATS_NAME_SUFFIX = "stats";
    public static final String VOLUME_STATES_NAME_SUFFIX = "states";
    public static final String VOLUME_UPGRADE_NAME_SUFFIX = "upgrade";
    public static final String VOLUME_RECORD_STREAM_NAME_SUFFIX = "record-streams";
    public static final String VOLUME_EVENTS_STREAM_NAME_SUFFIX = "events-streams";
    public static final Map<String, String> VOLUME_MOUNT_NAMES = Map.of(
            VOLUME_LOGS_NAME_SUFFIX, "hgcapp-%s".formatted(VOLUME_LOGS_NAME_SUFFIX),
            VOLUME_STATES_NAME_SUFFIX, "hgcapp-%s".formatted(VOLUME_STATES_NAME_SUFFIX),
            VOLUME_STATS_NAME_SUFFIX, "hgcapp-%s".formatted(VOLUME_STATS_NAME_SUFFIX),
            VOLUME_UPGRADE_NAME_SUFFIX, "hgcapp-%s".formatted(VOLUME_UPGRADE_NAME_SUFFIX),
            VOLUME_RECORD_STREAM_NAME_SUFFIX, "hgcapp-%s".formatted(VOLUME_RECORD_STREAM_NAME_SUFFIX),
            VOLUME_EVENTS_STREAM_NAME_SUFFIX, "hgcapp-%s".formatted(VOLUME_EVENTS_STREAM_NAME_SUFFIX));

    // components
    public static final String COMPONENT_ORBIT = "orbit";
    public static final String COMPONENT_CONSENSUS = "consensus";
    public static final String COMPONENT_HAPROXY = "haproxy";
    public static final String COMPONENT_NET_OP = "net-op";
    public static final String COMPONENT_CONFIG_MAP = "cm";
    public static final String COMPONENT_SECRET = "secret";
    public static final String COMPONENT_SERVICE = "svc";
    public static final String COMPONENT_POD = "pod";
    public static final String COMPONENT_PVC = "pvc";
    public static final String COMPONENT_DEPLOYMENT = "deployment";
    public static final String COMPONENT_ROLE_BINDING = "role-binding";
    public static final String COMPONENT_SERVICE_ACCOUNT = "sa";
    public static final String COMPONENT_ROLE = "role";
    public static final String COMPONENT_NAMESPACE = "ns";
}
