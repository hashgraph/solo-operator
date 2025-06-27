// SPDX-License-Identifier: Apache-2.0

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

/** This class contains unit tests for determining the phase of a consensus capsule from a pod's container status. */
public class ConsensusCapsuleContainerStatusTest {

    /**
     * Creates a running pod with the specified container name and readiness status.
     *
     * @param containerName The name of the container in the pod.
     * @param ready Whether the pod is ready or not.
     * @return A Pod object representing the pod
     */
    Pod running(final String containerName, final boolean ready) {
        return new PodBuilder()
                .withNewMetadata()
                .withName(containerName)
                .endMetadata()
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

    /**
     * Creates a terminated pod with the specified container name and exit code.
     *
     * @param containerName The name of the container in the pod.
     * @param exitCode The exit code of the terminated container.
     * @return A Pod object representing the pod
     */
    Pod terminated(final String containerName, final int exitCode) {
        return new PodBuilder()
                .withNewMetadata()
                .withName(containerName)
                .endMetadata()
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

    /**
     * Creates a pod with the specified container name that is waiting.
     *
     * @param containerName The name of the container in the pod.
     * @return A Pod object representing the pod
     */
    Pod waiting(final String containerName) {
        return new PodBuilder()
                .withNewMetadata()
                .withName(containerName)
                .endMetadata()
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

    /**
     * Creates a running pod with the specified container name.
     *
     * @param containerName The name of the container in the pod.
     * @return A Pod object representing the pod
     */
    Pod running(final String containerName) {
        return running(containerName, true);
    }

    /**
     * Creates a pending pod with the specified container name.
     *
     * @param containerName The name of the container in the pod.
     * @return A Pod object representing the pod
     */
    Pod pending(final String containerName) {
        return running(containerName, false);
    }

    /**
     * Creates a succeeded pod with the specified container name and exit code 0.
     *
     * @param containerName The name of the container in the pod.
     * @return A Pod object representing the pod
     */
    Pod succeeded(final String containerName) {
        return terminated(containerName, 0);
    }

    /**
     * Creates a failed pod with the specified container name and exit code 1.
     *
     * @param containerName The name of the container in the pod.
     * @return A Pod object representing the pod
     */
    Pod failed(final String containerName) {
        return terminated(containerName, 1);
    }

    /**
     * Creates a map of pods with their corresponding ConsensusCapsuleStatus phase.
     *
     * @param containerName The name of the consensus capsule container.
     * @return A Map containing Pod objects as keys and ConsensusCapsuleStatus phases as values
     */
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
