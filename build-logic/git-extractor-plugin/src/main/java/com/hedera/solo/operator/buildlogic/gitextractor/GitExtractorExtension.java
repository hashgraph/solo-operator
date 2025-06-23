// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.util.ArrayList;
import java.util.List;
import org.gradle.api.Project;
import org.gradle.api.file.Directory;
import org.gradle.api.tasks.SourceSet;

/**
 * The GitExtractorExtension class is responsible for configuring the extraction of Git repositories and their contents into a specified directory within the project's source set.
 */
public class GitExtractorExtension {
    protected Directory extractionDirectory;
    protected List<GitExtraction> extractions = new ArrayList<>();

    /**
     * Constructs a new GitExtractorExtension object, initializing the extraction directory based on the project and source set configurations.
     *
     * @param project The Gradle project instance.
     * @param sourceSet The source set for which the extraction is being configured.
     */
    public GitExtractorExtension(final Project project, final SourceSet sourceSet) {
        extractionDirectory = project.getLayout()
                .getBuildDirectory()
                .dir("generated/git-extractor/src/" + sourceSet.getName() + "/resources")
                .get();
    }

    /**
     * Adds a new Git extraction configuration to the list of extractions.
     *
     * @param repository The URL or path to the Git repository.
     * @param refspec A string specifying the reference (branch, tag, etc.) and other selection criteria for the repository contents.
     * @param source The relative path within the repository from which resources should be extracted.
     * @param destination The local directory within the project where the extracted resources should be placed.
     */
    public void addResources(
            final String repository, final String refspec, final String source, final String destination) {
        extractions.add(new GitExtraction(repository, refspec, source, destination));
    }
}
