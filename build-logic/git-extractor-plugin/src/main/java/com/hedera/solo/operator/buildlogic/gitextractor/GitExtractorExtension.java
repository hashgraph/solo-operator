// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.util.ArrayList;
import java.util.List;
import org.gradle.api.Project;
import org.gradle.api.file.Directory;
import org.gradle.api.tasks.SourceSet;

public class GitExtractorExtension {
    protected Directory extractionDirectory;
    protected List<GitExtraction> extractions = new ArrayList<>();

    public GitExtractorExtension(final Project project, final SourceSet sourceSet) {
        extractionDirectory = project.getLayout()
                .getBuildDirectory()
                .dir("generated/git-extractor/src/" + sourceSet.getName() + "/resources")
                .get();
    }

    public void addResources(
            final String repository, final String refspec, final String source, final String destination) {
        extractions.add(new GitExtraction(repository, refspec, source, destination));
    }
}
