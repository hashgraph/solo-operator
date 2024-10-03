/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.test.fixtures.snapshot;

import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import java.util.List;

public class SetupSnapshot {
    public static void enableCorednsLogs(final KubernetesClient client) {
        try {
            client.configMaps().inNamespace("kube-system").withName("coredns").edit(c -> c.edit()
                    .addToData("Corefile", enableCorednsLogs(c.getData().get("Corefile")))
                    .build());
        } catch (KubernetesClientException ignored) {
        }
    }

    private static String enableCorednsLogs(final String corefile) {
        List<String> lines = corefile.lines().toList();
        if (lines.stream().anyMatch((line -> line.strip().equals("log")))) {
            return corefile;
        }
        return corefile.replaceFirst(
                "(?<openBrace>\\{)(?<space>\\s*)(?<nonSpace>\\S)", "${openBrace}${space}log${space}${nonSpace}");
    }
}
