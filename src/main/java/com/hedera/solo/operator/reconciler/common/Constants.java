/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.reconciler.common;

import com.hedera.hashgraph.sdk.AccountId;
import com.hedera.hashgraph.sdk.PrivateKey;
import io.fabric8.kubernetes.api.model.PodSecurityContextBuilder;

public final class Constants {
    private Constants() {
        throw new UnsupportedOperationException("Utility class");
    }

    // defaults
    public static final String DEFAULT = "default";
    public static final String CONSENSUS_NODE = "consensus-node";
    public static final Long HEDERA_USER = 2000L;
    public static final Long HEDERA_GROUP = 2000L;
    public static final PodSecurityContextBuilder SECURITY_CONTEXT_BUILDER =
            new PodSecurityContextBuilder().withRunAsUser(HEDERA_USER).withRunAsGroup(HEDERA_GROUP);
    public static final int DEFAULT_FILE_MODE = 493; // 0755

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
    // FIXME all label key should have a prefix
    public static final String LABEL_APP = "app";
    public static final String LABEL_ACCOUNT_ID = "account-id";

    // key types
    public static final String KEY_TYPE_GOSSIP = "gossip";
    public static final String KEY_TYPE_GRPC_TLS = "grpc-tls";
}
