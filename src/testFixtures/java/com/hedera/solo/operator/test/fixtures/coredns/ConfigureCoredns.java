// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.coredns;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;
import java.time.Duration;
import java.util.HexFormat;
import java.util.List;

public class ConfigureCoredns {
    public static void configureCoredns(final KubernetesClient client) {
        try {
            // update corefile
            String[] file = new String[1];
            client.configMaps().inNamespace("kube-system").withName("coredns").edit(c -> c.edit()
                    .addToData(
                            "Corefile", file[0] = configureCoredns(c.getData().get("Corefile")))
                    .build());
            // add annotation to coredns pods with digest of corefile for faster reload
            String digest = HexFormat.of()
                    .formatHex(MessageDigest.getInstance("SHA-384").digest(file[0].getBytes()));
            Deployment deployment = client.resources(Deployment.class)
                    .inNamespace("kube-system")
                    .withName("coredns")
                    .edit(d -> d.edit()
                            .editSpec()
                            .editTemplate()
                            .editMetadata()
                            .addToAnnotations("corefile-digest", digest)
                            .endMetadata()
                            .endTemplate()
                            .endSpec()
                            .build());
            // wait for all pods running with annotation
            await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
                List<Pod> pods = client.pods()
                        .inNamespace("kube-system")
                        .withLabel("k8s-app", "kube-dns")
                        .list()
                        .getItems();
                long count = pods.stream()
                        .filter(p ->
                                digest.equals(p.getMetadata().getAnnotations().get("corefile-digest")))
                        .filter(p -> "Running".equals(p.getStatus().getPhase()))
                        .count();
                assertThat(count).isEqualTo((long) deployment.getSpec().getReplicas());
            });
        } catch (KubernetesClientException | NoSuchAlgorithmException ignored) {
        }
    }

    private static String configureCoredns(final String corefile) {
        String file = corefile;
        List<String> lines = file.lines().toList();
        // https://coredns.io/plugins/log/
        //        if (lines.stream().noneMatch((line -> line.strip().equals("log")))) {
        //            file = file.replaceFirst(
        //                    "(?<openBrace>\\{)(?<space>\\s*)(?<nonSpace>\\S)", "${openBrace}${space}log${space}${nonSpace}");
        //        }
        // https://coredns.io/plugins/cache/
        if (lines.stream().noneMatch((line -> line.strip().equals("disable success cluster.local")))) {
            // Hack to change the corefile installed by kind and used in testing without properly parsing the corefile.
            // This should go away if or when fixed upstream.
            // See also: https://github.com/kubernetes/kubernetes/pull/128359
            file = file.replaceFirst(
                    "(?<left>\\s*)cache 30(?<right>\\s*)",
                    "${left}cache 30 {${left}   disable success cluster.local${left}   disable denial cluster.local${left}}${right}");
        }
        return file;
    }
}
