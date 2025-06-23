// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.crdgenerator;

import io.fabric8.crd.generator.collector.CustomResourceCollector;
import io.fabric8.crdv2.generator.CRDGenerationInfo;
import io.fabric8.crdv2.generator.CRDGenerator;
import io.fabric8.crdv2.generator.CRDInfo;
import io.fabric8.crdv2.generator.CRDPostProcessor;
import io.fabric8.kubernetes.api.model.HasMetadata;
import java.io.File;
import java.net.MalformedURLException;
import java.net.URL;
import java.net.URLClassLoader;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;
import javax.inject.Inject;
import org.gradle.api.DefaultTask;
import org.gradle.api.file.FileCollection;
import org.gradle.api.tasks.CacheableTask;
import org.gradle.api.tasks.InputFiles;
import org.gradle.api.tasks.OutputDirectory;
import org.gradle.api.tasks.PathSensitive;
import org.gradle.api.tasks.PathSensitivity;
import org.gradle.api.tasks.TaskAction;

@CacheableTask
public class CrdGeneratorTask extends DefaultTask {
    @InputFiles
    @PathSensitive(PathSensitivity.RELATIVE)
    FileCollection filesToScan;

    @InputFiles
    @PathSensitive(PathSensitivity.RELATIVE)
    FileCollection classPathElements;

    @InputFiles
    @PathSensitive(PathSensitivity.RELATIVE)
    FileCollection postProcessorClassPath;

    @OutputDirectory
    final File outputDirectory;

    final CrdGeneratorExtension extension;

    @Inject
    public CrdGeneratorTask(final CrdGeneratorExtension extension) {
        filesToScan = extension.getFilesToScan();
        outputDirectory = extension.getOutputDir();
        classPathElements = extension.getClassPathElements();
        postProcessorClassPath = extension.getPostProcessorClassPath();
        this.extension = extension;
    }

    @TaskAction
    public void crdGenerate() throws Exception {
        CRDPostProcessor postProcessor = CRDPostProcessor.nullProcessor;
        if (extension.getPostProcessorClassName() != null) {
            try (URLClassLoader classLoader = new URLClassLoader(
                    urls(extension.getPostProcessorClassPath()), getClass().getClassLoader())) {
                postProcessor = extension
                        .getPostProcessorNewInstance()
                        .invoke(classLoader
                                .loadClass(extension.getPostProcessorClassName())
                                .asSubclass(CRDPostProcessor.class));
            }
        }
        CustomResourceCollector customResourceCollector = new CustomResourceCollector()
                .withClasspathElements(classPathElements.getFiles().stream()
                        .map(File::getAbsolutePath)
                        .collect(Collectors.toSet()))
                .withFilesToScan(filesToScan.getFiles())
                .withParentClassLoader(extension.getParentClassLoader());
        List<Class<? extends HasMetadata>> customResourceClasses = customResourceCollector.findCustomResourceClasses();

        outputDirectory.mkdirs();
        CRDGenerator crdGenerator = new CRDGenerator()
                .customResourceClasses(customResourceClasses)
                .inOutputDir(outputDirectory)
                .withPostProcessor(postProcessor);

        CRDGenerationInfo crdGenerationInfo = crdGenerator.detailedGenerate();

        Map<String, Map<String, CRDInfo>> details = crdGenerationInfo.getCRDDetailsPerNameAndVersion();
        details.keySet().forEach(name -> getLogger().info("Generating CRD {}", name));
    }

    URL[] urls(FileCollection classPath) throws MalformedURLException {
        List<URL> urls = new ArrayList<>();
        for (File file : classPath.getFiles()) {
            urls.add(file.toURI().toURL());
        }
        return urls.toArray(URL[]::new);
    }

    public FileCollection getFilesToScan() {
        return filesToScan;
    }

    public FileCollection getClassPathElements() {
        return classPathElements;
    }

    public FileCollection getPostProcessorClassPath() {
        return postProcessorClassPath;
    }

    public File getOutputDirectory() {
        return outputDirectory;
    }
}
