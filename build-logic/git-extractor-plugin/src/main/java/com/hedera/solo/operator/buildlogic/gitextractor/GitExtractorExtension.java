// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.util.ArrayList;
import java.util.List;

public class GitExtractorExtension {
    protected List<GitExtraction> extractions = new ArrayList<>();

    public void addResources(
            final String repository, final String refspec, final String source, final String destination) {
        extractions.add(new GitExtraction(repository, refspec, source, destination));
    }
}
