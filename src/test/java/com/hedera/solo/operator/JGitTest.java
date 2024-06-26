package com.hedera.solo.operator;

import com.google.common.jimfs.Configuration;
import com.google.common.jimfs.Jimfs;
import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.IOException;
import java.io.OutputStream;
import java.io.PipedInputStream;
import java.io.PipedOutputStream;
import java.lang.invoke.MethodHandles;
import java.nio.file.FileSystem;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.zip.ZipEntry;
import java.util.zip.ZipInputStream;
import org.eclipse.jgit.api.ArchiveCommand;
import org.eclipse.jgit.api.Git;
import org.eclipse.jgit.archive.ZipFormat;
import org.eclipse.jgit.lib.ObjectId;
import org.eclipse.jgit.lib.ObjectLoader;
import org.eclipse.jgit.lib.Ref;
import org.eclipse.jgit.lib.Repository;
import org.eclipse.jgit.revwalk.RevCommit;
import org.eclipse.jgit.revwalk.RevTree;
import org.eclipse.jgit.revwalk.RevWalk;
import org.eclipse.jgit.transport.TagOpt;
import org.eclipse.jgit.transport.URIish;
import org.eclipse.jgit.treewalk.TreeWalk;
import org.eclipse.jgit.treewalk.filter.PathFilter;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class JGitTest {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @TempDir
    public File tempDir;
    @Test
    public void test() throws Exception {
        try (//FileSystem fs = Jimfs.newFileSystem(Configuration.unix());
             Git git = Git.cloneRepository()
                .setURI("https://github.com/hashgraph/hedera-services")
                .setDirectory(tempDir)
//                .setBranch("v0.52.0-alpha.0")
//                .setNoCheckout(true)
//                .setDepth(1)
//                .setTagOption(TagOpt.AUTO_FOLLOW)
                .call()) {
            Repository repository = git.getRepository();
            log.info("Repository dir:  {}", repository.getDirectory());
            log.info("Repository bare: {}", repository.isBare());
            log.info("Repository head: {}", repository.resolve("HEAD"));
            log.info("Repository files: {}", Arrays.asList(tempDir.listFiles()));

            ObjectId version = repository.resolve("v0.52.0-alpha.0");

//            ObjectLoader loader = repository.open(version);
//            loader.copyTo(System.out);
//
//            try (RevWalk walk = new RevWalk(repository)) {
//                RevCommit commit = walk.parseCommit(version);
//                RevTree tree = walk.parseTree(commit.getTree().getId());
//                System.out.println("Found Tree: " + tree);
//                loader = repository.open(tree.getId());
//                loader.copyTo(System.out);
//
//                walk.dispose();
//            }

//            System.out.println("end commit");

            // a RevWalk allows to walk over commits based on some filtering that is defined
            try (RevWalk walk = new RevWalk(repository)) {
                RevCommit commit = walk.parseCommit(version);
                RevTree tree = commit.getTree();
                System.out.println("Having tree: " + tree);

                // now use a TreeWalk to iterate over all files in the Tree recursively
                // you can set Filters to narrow down the results if needed
                try (TreeWalk treeWalk = new TreeWalk(repository)) {
                    treeWalk.addTree(tree);
                    treeWalk.setRecursive(true);
                    treeWalk.setFilter(PathFilter.create("hedera-node/configuration/mainnet/"));
                    while (treeWalk.next()) {
                        System.out.println("found: " + treeWalk.getPathString());
                        repository.open(treeWalk.getObjectId(0)).copyTo(System.out);
                    }
                }
            }

            int i = 1;
        }
    }

    @Test
    public void test2() throws Exception {
        try (//FileSystem fs = Jimfs.newFileSystem(Configuration.unix());
             Git git = Git.init()
                     //.setURI("https://github.com/hashgraph/hedera-services")
                     .setDirectory(tempDir)
//                .setBranch("v0.52.0-alpha.0")
//                .setNoCheckout(true)
//                .setDepth(1)
//                .setTagOption(TagOpt.AUTO_FOLLOW)
                     .call()) {
            log.info("remotes: {}", git.remoteList().call());
            git.remoteAdd()
                    .setName("origin")
                    .setUri(new URIish("https://github.com/hashgraph/hedera-services"))
                    .call();
            log.info("remotes: {}", git.remoteList().call());
            // git.fetch().setRemote("origin").setInitialBranch("v0.52.0-alpha.0").setTagOpt(TagOpt.AUTO_FOLLOW).setCheckFetchedObjects(true).setDepth(1).call()
            // git.fetch().setRemote("origin").setRefSpecs("refs/tags/v0.52.0-alpha.0").setTagOpt(TagOpt.AUTO_FOLLOW).setDepth(1).call()

            git.fetch()
                    .setRemote("origin")
                    .setRefSpecs("refs/tags/v0.52.0-alpha.0")
//                    .setCheckFetchedObjects(true)
                    .setDepth(1)
                    .call();
            //git.fetch().setRefSpecs("refs/tags/v0.52.0-alpha.0:refs/tags/origin/v0.52.0-alpha.0").setDepth(1).call();
//            git.fetch().setInitialBranch("v0.52.0-alpha.0").setDepth(1).call();
            Repository repository = git.getRepository();
            ObjectId version =
//                    git.lsRemote().setTags(true).setHeads(false).setRemote("origin").call().stream().filter(x -> "refs/tags/v0.52.0-alpha.0".equals(x.getName())).findAny().get().getPeeledObjectId();
                    repository.resolve("v0.52.0-alpha.0");
            ArchiveCommand.registerFormat("zip", new ZipFormat());
            ByteArrayOutputStream baos = new ByteArrayOutputStream();
            git.archive()
                    .setFormat("zip")
                    .setOutputStream(baos)
                    .setPaths("hedera-node/configuration/mainnet/")
                    .setTree(version)
                    .call();
            String prefix = "/tmp/resources-123/";
            try (ZipInputStream zis = new ZipInputStream(new ByteArrayInputStream(baos.toByteArray()))) {

//                Thread.startVirtualThread(() -> {
//                            try {
//                            } catch (Exception e) {
//                                throw new RuntimeException(e);
//                            }
//                        });

//                Thread.sleep(999);
                ZipEntry ze;
                while((ze = zis.getNextEntry()) != null) {
                    Path path = Path.of(prefix, "hs/hn/cfg/mn").resolve(Path.of("hedera-node/configuration/mainnet").relativize(Path.of(ze.getName()))).normalize();
                    //
                    if (ze.isDirectory()) {
                        path.toFile().mkdirs();
//                        Path.of(prefix, ze.getName()).toFile().mkdirs();
                    } else {
                        var bytes = zis.readAllBytes();
                        System.out.printf("Entry: %s, bytes read: %s%n", ze.getName(), bytes.length);
                        Files.write(/*Path.of(prefix, ze.getName())*/path, bytes);
                    }
                }
            } finally {
                ArchiveCommand.unregisterFormat("zip");
            }
            if (true) { return; }
//            Repository repository = git.getRepository();
            log.info("Repository dir:  {}", repository.getDirectory());
            log.info("Repository bare: {}", repository.isBare());
            log.info("Repository head: {}", repository.resolve("HEAD"));
            log.info("Repository files: {}", Arrays.asList(tempDir.listFiles()));

//            ObjectId version = repository.resolve("v0.52.0-alpha.0");

//            ObjectLoader loader = repository.open(version);
//            loader.copyTo(System.out);
//
//            try (RevWalk walk = new RevWalk(repository)) {
//                RevCommit commit = walk.parseCommit(version);
//                RevTree tree = walk.parseTree(commit.getTree().getId());
//                System.out.println("Found Tree: " + tree);
//                loader = repository.open(tree.getId());
//                loader.copyTo(System.out);
//
//                walk.dispose();
//            }

//            System.out.println("end commit");

            // a RevWalk allows to walk over commits based on some filtering that is defined
            try (RevWalk walk = new RevWalk(repository)) {
                RevCommit commit = walk.parseCommit(version);
                RevTree tree = commit.getTree();
                System.out.println("Having tree: " + tree);

                // now use a TreeWalk to iterate over all files in the Tree recursively
                // you can set Filters to narrow down the results if needed
                try (TreeWalk treeWalk = new TreeWalk(repository)) {
                    treeWalk.addTree(tree);
                    treeWalk.setRecursive(true);
                    treeWalk.setFilter(PathFilter.create("hedera-node/configuration/mainnet/"));
                    while (treeWalk.next()) {
                        System.out.println("found: " + treeWalk.getPathString());
                        repository.open(treeWalk.getObjectId(0)).copyTo(System.out);
                    }
                }
            }

            int i = 1;
        }
    }
    @Test
    public void branch() throws Exception {
        try (//FileSystem fs = Jimfs.newFileSystem(Configuration.unix());
             Git git = Git.init()
                     //.setURI("https://github.com/hashgraph/hedera-services")
                     .setDirectory(tempDir)
//                .setBranch("v0.52.0-alpha.0")
//                .setNoCheckout(true)
//                .setDepth(1)
//                .setTagOption(TagOpt.AUTO_FOLLOW)
                     .call()) {
            log.info("remotes: {}", git.remoteList().call());
            git.remoteAdd()
                    .setName("origin")
                    .setUri(new URIish("https://github.com/hashgraph/hedera-services"))
                    .call();
            log.info("remotes: {}", git.remoteList().call());
            // git.fetch().setRemote("origin").setInitialBranch("v0.52.0-alpha.0").setTagOpt(TagOpt.AUTO_FOLLOW).setCheckFetchedObjects(true).setDepth(1).call()
            // git.fetch().setRemote("origin").setRefSpecs("refs/tags/v0.52.0-alpha.0").setTagOpt(TagOpt.AUTO_FOLLOW).setDepth(1).call()

            String ref =
//                    "refs/heads/develop";
                    "refs/tags/v0.52.0-alpha.0";
            Ref ref1 = git.fetch()
                    .setRemote("origin")
                    .setRefSpecs(ref)
//                    .setCheckFetchedObjects(true)
                    .setDepth(1)
                    .call()
                    .getAdvertisedRef(ref);
            ObjectId version = ref1.getObjectId(); //ref1.isPeeled() ? ref1.getPeeledObjectId() : ref1.getObjectId();
                    log.info("version: {}", version.toString());
            //git.fetch().setRefSpecs("refs/tags/v0.52.0-alpha.0:refs/tags/origin/v0.52.0-alpha.0").setDepth(1).call();
//            git.fetch().setInitialBranch("v0.52.0-alpha.0").setDepth(1).call();
            Repository repository = git.getRepository();
//            ObjectId version =
////                    git.lsRemote().setTags(true).setHeads(false).setRemote("origin").call().stream().filter(x -> "refs/tags/v0.52.0-alpha.0".equals(x.getName())).findAny().get().getPeeledObjectId();
//                    repository.resolve("develop");
            ArchiveCommand.registerFormat("zip", new ZipFormat());
            ByteArrayOutputStream baos = new ByteArrayOutputStream();
            git.archive()
                    .setFormat("zip")
                    .setOutputStream(baos)
                    .setPaths("hedera-node/configuration/mainnet/")
                    .setTree(version)
                    .call();
            String prefix = "/tmp/resources-123/";
            try (ZipInputStream zis = new ZipInputStream(new ByteArrayInputStream(baos.toByteArray()))) {

//                Thread.startVirtualThread(() -> {
//                            try {
//                            } catch (Exception e) {
//                                throw new RuntimeException(e);
//                            }
//                        });

//                Thread.sleep(999);
                ZipEntry ze;
                while((ze = zis.getNextEntry()) != null) {
                    Path path = Path.of(prefix, "hs/hn/cfg/mn").resolve(Path.of("hedera-node/configuration/mainnet").relativize(Path.of(ze.getName()))).normalize();
                    //
                    if (ze.isDirectory()) {
                        path.toFile().mkdirs();
//                        Path.of(prefix, ze.getName()).toFile().mkdirs();
                    } else {
                        var bytes = zis.readAllBytes();
                        System.out.printf("Entry: %s, bytes read: %s%n", ze.getName(), bytes.length);
                        Files.write(/*Path.of(prefix, ze.getName())*/path, bytes);
                    }
                }
            } finally {
                ArchiveCommand.unregisterFormat("zip");
            }
            if (true) { return; }
//            Repository repository = git.getRepository();
            log.info("Repository dir:  {}", repository.getDirectory());
            log.info("Repository bare: {}", repository.isBare());
            log.info("Repository head: {}", repository.resolve("HEAD"));
            log.info("Repository files: {}", Arrays.asList(tempDir.listFiles()));

//            ObjectId version = repository.resolve("v0.52.0-alpha.0");

//            ObjectLoader loader = repository.open(version);
//            loader.copyTo(System.out);
//
//            try (RevWalk walk = new RevWalk(repository)) {
//                RevCommit commit = walk.parseCommit(version);
//                RevTree tree = walk.parseTree(commit.getTree().getId());
//                System.out.println("Found Tree: " + tree);
//                loader = repository.open(tree.getId());
//                loader.copyTo(System.out);
//
//                walk.dispose();
//            }

//            System.out.println("end commit");

            // a RevWalk allows to walk over commits based on some filtering that is defined
            try (RevWalk walk = new RevWalk(repository)) {
                RevCommit commit = walk.parseCommit(version);
                RevTree tree = commit.getTree();
                System.out.println("Having tree: " + tree);

                // now use a TreeWalk to iterate over all files in the Tree recursively
                // you can set Filters to narrow down the results if needed
                try (TreeWalk treeWalk = new TreeWalk(repository)) {
                    treeWalk.addTree(tree);
                    treeWalk.setRecursive(true);
                    treeWalk.setFilter(PathFilter.create("hedera-node/configuration/mainnet/"));
                    while (treeWalk.next()) {
                        System.out.println("found: " + treeWalk.getPathString());
                        repository.open(treeWalk.getObjectId(0)).copyTo(System.out);
                    }
                }
            }

            int i = 1;
        }
    }
}
