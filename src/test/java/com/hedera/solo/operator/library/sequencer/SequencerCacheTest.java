// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no1;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no2;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no3;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no4;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.orbit1;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.orbit2;
import static org.assertj.core.api.AssertionsForInterfaceTypes.assertThat;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import java.util.List;
import org.assertj.core.api.AssertionsForClassTypes;
import org.junit.jupiter.api.Test;

/** A test class for the SequencerCache class which manages a cache of sequenced items. */
class SequencerCacheTest {
    @Test
    void addItem() {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();
        assertThat(cache.indexOf(no1)).isEqualTo(-1);
        assertThat(cache.indexOf(null)).isEqualTo(-1);

        cache.addItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());

        // Adding the same item again would be idempotent
        cache.addItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.indexOf(no1)).isEqualTo(0);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());

        cache.addItem(no3);
        assertThat(cache.size(orbit1)).isEqualTo(2);
        assertThat(cache.indexOf(no2)).isEqualTo(-1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());

        cache.addItem(no2);
        assertThat(cache.size(orbit1)).isEqualTo(3);
        assertThat(cache.indexOf(no2)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());

        // different orbit
        cache.addItem(no4);
        assertThat(cache.size(orbit2)).isEqualTo(1);
        assertThat(cache.findFirst(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());
        assertThat(cache.findLast(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());
    }

    @Test
    void removeItem() {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();
        assertThat(cache.removeItem(no1)).isNull();

        cache.addItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());

        cache.addItem(no3);
        assertThat(cache.size(orbit1)).isEqualTo(2);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());

        // different orbit
        cache.addItem(no4);
        assertThat(cache.size(orbit2)).isEqualTo(1);
        assertThat(cache.findFirst(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());
        assertThat(cache.findLast(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());

        cache.removeItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());

        // other namespace should remain intact
        assertThat(cache.size(orbit2)).isEqualTo(1);
        assertThat(cache.findFirst(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());
        assertThat(cache.findLast(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());
    }

    @Test
    void findFirstAndLast() {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();
        assertThat(cache.findFirst("invalid")).isNull();
        assertThat(cache.findLast("invalid")).isNull();

        cache.addItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());

        // remove and then find again
        cache.removeItem(no1);
        assertThat(cache.findFirst(orbit1)).isNull();
        assertThat(cache.findLast(orbit1)).isNull();
    }

    @Test
    void findItem() {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();
        cache.addItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());

        cache.addItem(no3);
        assertThat(cache.size(orbit1)).isEqualTo(2);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());

        // different orbit
        cache.addItem(no4);
        assertThat(cache.size(orbit2)).isEqualTo(1);
        assertThat(cache.findFirst(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());
        assertThat(cache.findLast(orbit2).uid()).isEqualTo(no4.getMetadata().getUid());

        assertThat(cache.findItem(no2)).isNull();
        assertThat(cache.findItem(no1)).isNotNull();
        assertThat(cache.findItem(no3)).isNotNull();
        assertThat(cache.findItem(no4)).isNotNull();
    }

    @Test
    void items() {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();
        AssertionsForClassTypes.assertThat(cache.items(orbit1).size()).isEqualTo(0);
        AssertionsForClassTypes.assertThat(cache.size(orbit1)).isEqualTo(0);

        cache.addItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());

        cache.addItem(no3);
        assertThat(cache.size(orbit1)).isEqualTo(2);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());

        List<Sequenced> items = cache.items(orbit1);
        assertThat(items.size()).isEqualTo(2);
        assertThat(items.getFirst().uid().equals(no1.getMetadata().getUid())).isTrue();
        assertThat(items.getLast().uid().equals(no3.getMetadata().getUid())).isTrue();
    }

    @Test
    void isFirstInQueue() {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();
        AssertionsForClassTypes.assertThat(cache.items(orbit1).size()).isEqualTo(0);
        AssertionsForClassTypes.assertThat(cache.size(orbit1)).isEqualTo(0);

        cache.addItem(no1);
        assertThat(cache.size(orbit1)).isEqualTo(1);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());

        cache.addItem(no3);
        assertThat(cache.size(orbit1)).isEqualTo(2);
        assertThat(cache.findFirst(orbit1).uid()).isEqualTo(no1.getMetadata().getUid());
        assertThat(cache.findLast(orbit1).uid()).isEqualTo(no3.getMetadata().getUid());

        assertThat(cache.isFirstInQueue(no1)).isTrue();
        assertThat(cache.isFirstInQueue(no3)).isFalse();
        assertThat(cache.isFirstInQueue(no4)).isFalse();
    }

    @Test
    void getCurrentlyExecuting() {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();
        assertThat(cache.getCurrentlyExecuting("invalid")).isNull();

        cache.addItem(no1);
        assertThat(cache.getCurrentlyExecuting(orbit1)).isNull();
        cache.markAsExecuting(no1);
        assertThat(cache.getCurrentlyExecuting(orbit1)).isNotNull();
        assertThat(cache.getCurrentlyExecuting(orbit1).uid())
                .isEqualTo(no1.getMetadata().getUid());
    }

    @Test
    void prune() throws InterruptedException {
        final SequencerCache<NetworkOperation> cache = new SequencerCache<>();

        cache.addItem(no1);
        cache.addItem(no2);
        cache.addItem(no3);
        cache.prune(List.of(no2, no3));
        assertThat(cache.contains(no1)).isFalse();
    }
}
