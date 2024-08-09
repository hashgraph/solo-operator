package com.hedera.solo.operator.reconciler.common;

import com.hedera.hashgraph.sdk.AccountId;
import com.hedera.hashgraph.sdk.PrivateKey;

public final class Constants {
    private Constants() {
        throw new UnsupportedOperationException("Utility class");
    }

    // HEDERA related
    public final static int HEDERA_ACCOUNT_START = 3;
    public final static AccountId OPERATOR_ACCOUNT_ID = AccountId.fromString("0.0.2");
    public final static PrivateKey OPERATOR_PRIVATE_KEY = PrivateKey.fromString(
            "302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137");

    // Helm chart related
    public final static String MINIO_CHART_REPO = "minio-operator";
    public final static String MINIO_CHART_REPO_URL = "https://operator.min.io";
    public final static String MINIO_OPERATOR_CHART = String.format("%s/operator", MINIO_CHART_REPO);
    public final static String MINIO_TENANT_CHART = String.format("%s/tenant", MINIO_CHART_REPO);
    public final static String MINIO_OPERATOR_RELEASE = "minio-operator";
    public final static String MINIO_TENANT_RELEASE = "minio-tenant";
}
