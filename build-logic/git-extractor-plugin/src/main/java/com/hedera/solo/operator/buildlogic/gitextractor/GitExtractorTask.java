// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collection;
import java.util.List;
import java.util.stream.Collectors;
import java.util.zip.ZipEntry;
import java.util.zip.ZipInputStream;
import javax.inject.Inject;
import org.eclipse.jgit.api.ArchiveCommand;
import org.eclipse.jgit.api.Git;
import org.eclipse.jgit.archive.ZipFormat;
import org.eclipse.jgit.transport.FetchResult;
import org.eclipse.jgit.transport.URIish;
import org.gradle.api.DefaultTask;
import org.gradle.api.file.Directory;
import org.gradle.api.tasks.CacheableTask;
import org.gradle.api.tasks.Input;
import org.gradle.api.tasks.OutputDirectories;
import org.gradle.api.tasks.OutputDirectory;
import org.gradle.api.tasks.TaskAction;

/**
 * The GitExtractorTask class is a Gradle task that extracts code from a git repository.
 */
@CacheableTask
public class GitExtractorTask extends DefaultTask {
    @Input
    final List<GitExtraction> extractions;
    @OutputDirectory
    final Directory extractionDirectory;
    @OutputDirectories
    final Collection<Path> outputDirectories;

    /**
     * Constructor for GitExtractorTask that initializes the task with the provided extension.
     *
     * @param extension The {@link GitExtractorExtension} containing extraction specifications and directory.
     */
    @Inject
    public GitExtractorTask(final GitExtractorExtension extension) {
        extractions = extension.extractions;
        extractionDirectory = extension.extractionDirectory;
        outputDirectories = extension.extractions.stream()
                .map(e -> extractionDirectory
                        .dir(e.destination())
                        .getAsFile()
                        .toPath())
                .collect(Collectors.toSet());
    }

    /**
     * The main action method for the task, which triggers the extraction process.
     *
     * @throws Exception If an error occurs during the extraction process, such as issues with the git repository or file operations.
     */
    @TaskAction
    public void gitExtractor() throws Exception {
        for (GitExtraction extraction : extractions) {
            extract(extraction);
        }
    }

    /**
     * Extracts a single {@link GitExtraction} by cloning the repository, fetching the specified refspec, and then archiving it.
     *
     * @param extraction The {@link GitExtraction} object containing the details of what to extract.
     * @throws Exception If an error occurs during the extraction process, such as issues with the git repository or file operations.
     */
    void extract(final GitExtraction extraction) throws Exception {
        try (TempDirectory tempDirectory = new TempDirectory("sologitextractor");
                Git git = Git.init().setDirectory(tempDirectory.path.toFile()).call()) {
            git.remoteAdd()
                    .setName("origin")
                    .setUri(new URIish(extraction.repository()))
                    .call();
            FetchResult fetchResult = git.fetch()
                    .setRemote("origin")
                    .setRefSpecs(extraction.refspec())
                    .setDepth(1)
                    .call();
            ArchiveCommand.registerFormat("zip", new ZipFormat());
            ByteArrayOutputStream baos = new ByteArrayOutputStream();
            try (OutputStream os = git.archive()
                            .setFormat("zip")
                            .setOutputStream(baos)
                            .setPaths(extraction.source())
                            .setTree(fetchResult
                                    .getAdvertisedRef(extraction.refspec())
                                    .getObjectId())
                            .call();
                    ZipInputStream zis = new ZipInputStream(new ByteArrayInputStream(baos.toByteArray()))) {
                ZipEntry ze;
                Path destination = extractionDirectory
                        .dir(extraction.destination())
                        .getAsFile()
                        .toPath();
                while ((ze = zis.getNextEntry()) != null) {
                    Path path = destination
                            .resolve(Path.of(extraction.source()).relativize(Path.of(ze.getName())))
                            .normalize();
                    if (ze.isDirectory()) {
                        path.toFile().mkdirs();
                    } else {
                        var bytes = zis.readAllBytes();
                        Files.write(path, bytes);
                    }
                }
            } finally {
                ArchiveCommand.unregisterFormat("zip");
            }
        }
    }

    /**
     * Returns the list of {@link GitExtraction} objects configured for this task.
     *
     * @return The list of {@link GitExtraction} objects
     */
    public List<GitExtraction> getExtractions() {
        return extractions;
    }

    /**
     * Returns the directory where extracted files are stored.
     *
     * @return The extraction directory as a {@link Directory}
     */
    public Directory getExtractionDirectory() {
        return extractionDirectory;
    }

    /**
     * Returns the collection of output directories created by this task.
     *
     * @return A collection of paths to the output directories
     */
    public Collection<Path> getOutputDirectories() {
        return outputDirectories;
    }
}
