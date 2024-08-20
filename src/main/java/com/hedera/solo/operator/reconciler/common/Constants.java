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
import java.util.List;

public final class Constants {
    private Constants() {
        throw new UnsupportedOperationException("Utility class");
    }

    // HEDERA related
    public static final int HEDERA_ACCOUNT_START = 3;
    public static final AccountId OPERATOR_ACCOUNT_ID = AccountId.fromString("0.0.2");
    public static final PrivateKey OPERATOR_PRIVATE_KEY = PrivateKey.fromString(
            "302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137");

    // Helm chart related
    public static final String MINIO_CHART_REPO = "minio-operator";
    public static final String MINIO_CHART_REPO_URL = "https://operator.min.io";
    public static final String MINIO_OPERATOR_CHART = String.format("%s/operator", MINIO_CHART_REPO);
    public static final String MINIO_TENANT_CHART = String.format("%s/tenant", MINIO_CHART_REPO);
    public static final String MINIO_OPERATOR_RELEASE = "minio-operator";
    public static final String MINIO_TENANT_RELEASE = "minio-tenant";

    // Templates
    public static final List<String> NODE_KEYS_FILENAMES_TEMPLATE =
            List.of("a-private-%s.pem", "s-private-%s.pem", "hedera-%s.key", "hedera-%s.crt");
}
