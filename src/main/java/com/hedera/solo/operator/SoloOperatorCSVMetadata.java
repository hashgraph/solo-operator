// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator;

import io.quarkiverse.operatorsdk.annotations.CSVMetadata;
import io.quarkiverse.operatorsdk.annotations.SharedCSVMetadata;

/** Metadata class for the Solo Operator CSV (Cluster Service Version) metadata. */
@CSVMetadata(bundleName = "solo-operator")
public class SoloOperatorCSVMetadata implements SharedCSVMetadata {}
