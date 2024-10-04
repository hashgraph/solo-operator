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

package com.hedera.solo.operator.test.unit;

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleReconciler.CONSENSUS_CONTAINER_NAME;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleReconciler.phase;
import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import io.fabric8.kubernetes.api.model.ContainerStateBuilder;
import io.fabric8.kubernetes.api.model.ContainerStateRunning;
import io.fabric8.kubernetes.api.model.ContainerStateTerminatedBuilder;
import io.fabric8.kubernetes.api.model.ContainerStateWaiting;
import io.fabric8.kubernetes.api.model.ContainerStatusBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodBuilder;
import java.util.Map;
import org.junit.jupiter.api.Test;

public class ConsensusCapsuleContainerStatusTest {
    Pod running(final String containerName, final boolean ready) {
        return new PodBuilder()
                .editStatus()
                .withContainerStatuses(new ContainerStatusBuilder()
                        .withName(containerName)
                        .withState(new ContainerStateBuilder()
                                .withRunning(new ContainerStateRunning())
                                .build())
                        .withReady(ready)
                        .build())
                .endStatus()
                .build();
    }

    Pod terminated(final String containerName, final int exitCode) {
        return new PodBuilder()
                .editStatus()
                .withContainerStatuses(new ContainerStatusBuilder()
                        .withName(containerName)
                        .withState(new ContainerStateBuilder()
                                .withTerminated(new ContainerStateTerminatedBuilder()
                                        .withExitCode(exitCode)
                                        .build())
                                .build())
                        .build())
                .endStatus()
                .build();
    }

    Pod waiting(final String containerName) {
        return new PodBuilder()
                .editStatus()
                .withContainerStatuses(new ContainerStatusBuilder()
                        .withName(containerName)
                        .withState(new ContainerStateBuilder()
                                .withWaiting(new ContainerStateWaiting())
                                .build())
                        .build())
                .endStatus()
                .build();
    }

    Pod running(final String containerName) {
        return running(containerName, true);
    }

    Pod pending(final String containerName) {
        return running(containerName, false);
    }

    Pod succeeded(final String containerName) {
        return terminated(containerName, 0);
    }

    Pod failed(final String containerName) {
        return terminated(containerName, 1);
    }

    Map<Pod, ConsensusCapsuleStatus.Phase> pods(final String containerName) {
        return Map.of(
                running(containerName), ConsensusCapsuleStatus.Phase.Running,
                pending(containerName), ConsensusCapsuleStatus.Phase.Pending,
                succeeded(containerName), ConsensusCapsuleStatus.Phase.Succeeded,
                failed(containerName), ConsensusCapsuleStatus.Phase.Failed,
                waiting(containerName), ConsensusCapsuleStatus.Phase.Unknown);
    }

    final Map<Pod, ConsensusCapsuleStatus.Phase> validPods = pods(CONSENSUS_CONTAINER_NAME);

    @Test
    void determinePhaseOfCapsuleFromPod() {
        for (Map.Entry<Pod, ConsensusCapsuleStatus.Phase> entry : validPods.entrySet()) {
            assertThat(phase(entry.getKey())).isEqualTo(entry.getValue());
        }
    }

    @Test
    void requireMatchingContainerName() {
        for (Pod pod : pods(CONSENSUS_CONTAINER_NAME + "-invalid").keySet()) {
            assertThat(phase(pod)).isEqualTo(ConsensusCapsuleStatus.Phase.Unknown);
        }
    }

    @Test
    void requireExactlyOnePod() {
        assertThat(phase()).isEqualTo(ConsensusCapsuleStatus.Phase.Unknown);
        for (Pod pod : validPods.keySet()) {
            assertThat(phase(pod, pod)).isEqualTo(ConsensusCapsuleStatus.Phase.Unknown);
        }
    }
}
