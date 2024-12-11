// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Collection;
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
import org.gradle.api.tasks.OutputDirectories;
import org.gradle.api.tasks.TaskAction;

@CacheableTask
public class GitExtractorTask extends DefaultTask {
    GitExtractorExtension extension;

    @OutputDirectories
    Collection<Path> outputDirectories;

    public Collection<Path> getOutputDirectories() {
        return outputDirectories;
    }

    @Inject
    public GitExtractorTask(GitExtractorExtension extension) {
        this.extension = extension;
        outputDirectories = extension.extractions.stream()
                .map(e -> extension.extractionDirectory.dir(e.destination()).getAsFile().toPath())
                .collect(Collectors.toSet());
    }

    @TaskAction
    public void gitExtractor() throws Exception {
        for (GitExtraction extraction : extension.extractions) {
            extract(extraction);
        }
    }

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
                Path destination = extension.extractionDirectory
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
}
