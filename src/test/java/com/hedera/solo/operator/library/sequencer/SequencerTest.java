// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no1;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no2;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no3;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no4;
import static org.assertj.core.api.AssertionsForClassTypes.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.Condition;
import java.time.Duration;
import java.util.List;
import org.jboss.logging.Logger;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

class SequencerTest {
    private final Logger log = Logger.getLogger(SequencerTest.class.getName());
    Sequencer<NetworkOperation> sequencer;

    private static NetworkOperation setStatus(NetworkOperation resource, String phase, List<Condition> conditions) {
        return edit(resource)
                .with(r -> r.editStatus()
                        .withPhase(NetworkOperationStatus.Phase.valueOf(phase))
                        .addAllToConditions(conditions)
                        .endStatus())
                .build();
    }

    @BeforeEach
    void setUp() {
        sequencer = new Sequencer<>(() -> List.of(no1, no2, no3));
    }

    @Test
    void computeReconcileDelay() {
        // when there is no item, it would return the recommended reconcile delay in the request
        Duration d = sequencer.computeReconcileDelay(
                new SequencerRequest<>(no1, Duration.ofMillis(200), Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ofMillis(200));

        // check would trigger adding all items
        sequencer.checkNext(new SequencerRequest<>(no1));

        // for first item, it would return the recommended delay in the request
        d = sequencer.computeReconcileDelay(
                new SequencerRequest<>(no1, Duration.ofMillis(200), Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ofMillis(200));

        // for other items, it would return 1/3rd of the total execution time of the items before the queried one
        d = sequencer.computeReconcileDelay(
                new SequencerRequest<>(no3, Duration.ofMillis(200), Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ofMillis((long) Math.floor(100 * 2 * 0.3)));

        // if recommended delay is smaller, return the recommended delay even if the computed reconcile delay is larger
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, Duration.ofMillis(10), Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ofMillis(10));

        // check default
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, null, null));
        assertThat(d).isEqualTo(Sequencer.DEFAULT_RECONCILIATION_DELAY);

        // check zero, null, negative and too large recommended reconcile time
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, null, Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ofMillis((long) Math.floor(100 * 2 * 0.3)));
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, Duration.ZERO, Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ZERO);
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, Duration.ofMillis(-1), Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ofMillis((long) Math.floor(100 * 2 * 0.3)));
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(
                no3, Duration.ofMillis(Sequencer.MAX_RECONCILIATION_DELAY.toMillis() * 2), Duration.ofMillis(100)));
        assertThat(d).isEqualTo(Duration.ofMillis((long) Math.floor(100 * 2 * 0.3)));

        // check zero, null, negative or too large processing time
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, Duration.ofMillis(10000), null));
        assertThat(d).isEqualTo(Duration.ofMillis((long) Math.floor(
                Sequencer.DEFAULT_ESTIMATED_PROCESSING_TIME.multipliedBy(2).toMillis() * 0.3)));
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, Duration.ofMillis(10000), Duration.ZERO));
        assertThat(d).isEqualTo(Duration.ofMillis((long) Math.floor(
                Sequencer.DEFAULT_ESTIMATED_PROCESSING_TIME.multipliedBy(2).toMillis() * 0.3)));
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(no3, Duration.ofMillis(200), Duration.ofMillis(-1)));
        assertThat(d).isEqualTo(Duration.ofMillis(200));
        d = sequencer.computeReconcileDelay(new SequencerRequest<>(
                no3, Duration.ofDays(1), Duration.ofMillis(Sequencer.MAX_ESTIMATED_PROCESSING_TIME.toMillis() * 100)));
        assertThat(d).isEqualTo(Duration.ofMillis((long) Math.floor(
                Sequencer.MAX_ESTIMATED_PROCESSING_TIME.multipliedBy(2).toMillis() * 0.3)));
    }

    @Test
    void checkNext() {
        SequencerResponse<NetworkOperation> response = sequencer.checkNext(new SequencerRequest<>(no1));
        assertThat(response.firstInQueue()).isTrue();
        assertThat(response.reconcileDelay()).isEqualTo(Duration.ZERO);

        response = sequencer.checkNext(new SequencerRequest<>(no3));
        assertThat(response.firstInQueue()).isFalse();

        // invalid item would return desired resource with failed status
        response = sequencer.checkNext(new SequencerRequest<>(no4));
        assertThat(response.firstInQueue()).isFalse();
    }

    @Test
    void markAsCompleted() {
        assertThrows(NullPointerException.class, () -> sequencer.markAsCompleted(null));
        assertThrows(NullPointerException.class, () -> sequencer.markAsCompleted(new SequencerRequest<>(null)));
        assertThat(sequencer.markAsCompleted(new SequencerRequest<>(no1))).isFalse();

        sequencer.checkNext(new SequencerRequest<>(no1));
        sequencer.checkNext(new SequencerRequest<>(no2));
        assertThrows(SequencerException.class, () -> sequencer.markAsCompleted(new SequencerRequest<>(no2)));

        assertThat(sequencer.markAsCompleted(new SequencerRequest<>(no1))).isTrue();
    }

    @Test
    void markAsDeleted() {
        assertThrows(NullPointerException.class, () -> sequencer.markAsDeleted(null));
        assertThrows(NullPointerException.class, () -> sequencer.markAsDeleted(new SequencerRequest<>(null)));
        assertThat(sequencer.markAsDeleted(new SequencerRequest<>(no1))).isFalse();

        sequencer.refresh();
        assertThat(sequencer.markAsDeleted(new SequencerRequest<>(no2))).isTrue();
        assertThat(sequencer.markAsDeleted(new SequencerRequest<>(no4))).isFalse();
    }

    @Test
    void findOrRefresh() {
        assertThrows(NullPointerException.class, () -> sequencer.findOrRefresh(null));
        Sequenced item = sequencer.findOrRefresh(no1);
        assertThat(item).isNotNull();
        assertThat(item.name()).isEqualTo(no1.getMetadata().getName());
        assertThat(item.uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(item.namespace()).isEqualTo(no1.getMetadata().getNamespace());
        assertThat(item.creationTime().toString()).isEqualTo(no1.getMetadata().getCreationTimestamp());
    }
}
