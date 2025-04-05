// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.Serializable;

public record GitExtraction(String repository, String refspec, String source, String destination) implements Serializable {}
