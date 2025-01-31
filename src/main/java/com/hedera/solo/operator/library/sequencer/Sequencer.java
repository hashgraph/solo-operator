// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.smallrye.mutiny.tuples.Functions;
import java.time.Duration;
import java.util.List;
import java.util.Objects;
import java.util.function.Supplier;

/**
 * Sequencer is responsible for ordering the processing of custom resources and computing the reconcile delay.
 *
 * @param <T> custom resource type
 */
public final class Sequencer<T extends HasMetadata> {
    private final SequencerCache<T> cache = new SequencerCache<>();
    private final Supplier<List<T>> activeResources;
    private final Functions.Function3<T, String, List<Condition>, T> setStatus;

    public static final Duration MAX_RECONCILIATION_DELAY = Duration.ofHours(1);
    public static final Duration DEFAULT_RECONCILIATION_DELAY = Duration.ofMillis(100);

    public static final Duration MAX_ESTIMATED_PROCESSING_TIME = Duration.ofHours(1);
    public static final Duration DEFAULT_ESTIMATED_PROCESSING_TIME = Duration.ofSeconds(1);

    public static final String PENDING = "Pending";
    public static final String SUCCEEDED = "Succeeded";
    public static final String FAILED = "Failed";

    public Sequencer(
            final Supplier<List<T>> activeResources,
            final Functions.Function3<T, String, List<Condition>, T> setStatus) {
        Objects.requireNonNull(activeResources, "activeResources parameter cannot be null");
        this.activeResources = activeResources;
        this.setStatus = setStatus;
    }

    /** Refresh the cache using data from the Kubernetes API server using the provided supplier. */
    public synchronized void refresh() {
        cache.prune();

        List<T> activeItems = activeResources.get();
        if (activeItems == null || activeItems.isEmpty()) {
            return;
        }

        for (T item : activeItems) {
            cache.addItem(item);
        }
    }

    /**
     * Find the resource in the cache or refresh the cache if the resource is not found.
     *
     * <p>Here the lazy loading of the cache is performed when it is accessed for the first time. For subsequent access,
     * either the item is found in the cache or it is refreshed again if not found
     *
     * @param resource custom resource
     */
    public synchronized Sequenced findOrRefresh(final T resource) {
        Objects.requireNonNull(resource, "resource cannot be null");
        Objects.requireNonNull(resource.getMetadata(), "metadata cannot be null");
        Objects.requireNonNull(resource.getMetadata().getNamespace(), "namespace cannot be null");
        Objects.requireNonNull(resource.getMetadata().getUid(), "uid cannot be null");
        Objects.requireNonNull(resource.getMetadata().getName(), "name cannot be null");

        Sequenced sequencedItem = cache.findItem(resource);
        if (sequencedItem != null) {
            return sequencedItem;
        }

        this.refresh(); // lazy refresh
        return cache.findItem(resource);
    }

    /**
     * Compute the reconcile delay based on the recommended delay, estimated processing time and the items in the queue.
     *
     * <p>Given a total of n items in the queue, it tries to estimate the processing time of all the earlier resources
     * in the queue and then selects 30% of the execution time as the reconciliation delay. This value is then compared
     * with the delay recommended by the reconciler and the larger value is selected.
     *
     * @param resource custom resource
     * @param recommendedDelay recommended delay
     * @param estimatedProcessingTime estimated processing time
     * @return the reconcile delay based on the items in the queue and recommended metadata.
     */
    private synchronized Duration computeReconcileDelay(
            final T resource, final Duration recommendedDelay, final Duration estimatedProcessingTime) {

        final Duration processingTime;
        final Duration reconcileDelay;

        if (estimatedProcessingTime == null
                || estimatedProcessingTime.isNegative()
                || estimatedProcessingTime.isZero()) {
            processingTime = DEFAULT_ESTIMATED_PROCESSING_TIME;
        } else if (estimatedProcessingTime.compareTo(MAX_ESTIMATED_PROCESSING_TIME) > 0) {
            processingTime = MAX_ESTIMATED_PROCESSING_TIME;
        } else {
            processingTime = estimatedProcessingTime;
        }

        if (recommendedDelay == null || recommendedDelay.isNegative()) {
            reconcileDelay = DEFAULT_RECONCILIATION_DELAY;
        } else if (recommendedDelay.compareTo(MAX_RECONCILIATION_DELAY) > 0) {
            reconcileDelay = MAX_RECONCILIATION_DELAY;
        } else {
            reconcileDelay = recommendedDelay;
        }

        final int index = cache.indexOf(resource);
        if (index <= 0) {
            return reconcileDelay;
        }

        // take 30% of the total execution time of all the items before this item
        final long totalExecutionTime = processingTime.multipliedBy(index).toMillis();
        final Duration computedReconcileDelay = Duration.ofMillis((long) Math.floor(totalExecutionTime * 0.3));

        // choose the smaller of the computed delay and recommended delay
        if (computedReconcileDelay.compareTo(reconcileDelay) < 0) {
            return computedReconcileDelay;
        }

        return reconcileDelay;
    }

    /**
     * Compute the reconcile delay based on the recommended delay, estimated processing time and the items in the queue.
     *
     * @param request request including the resource, recommended delay and estimated processing time
     * @return the reconcile delay based on the items in the queue and recommended metadata.
     */
    public synchronized Duration computeReconcileDelay(final SequencerRequest<T> request) {
        return computeReconcileDelay(request.resource(), request.reconcileDelay(), request.processingTime());
    }

    /**
     * Check whether the resource is already processed based on the cache.
     *
     * <p>Reconciler must call this before it starts processing the resource.
     *
     * @param resource custom resource
     */
    public synchronized boolean isProcessed(T resource) {
        Objects.requireNonNull(resource, "resource cannot be null");
        return cache.isProcessed(resource);
    }

    /**
     * Check whether the resource is first in queue to be processed.
     *
     * <p>Reconciler must call this before it processes the resource.
     *
     * @param request the request including the resource and other metadata
     * @return the response including the resource, reconcile delay, sequenced item and whether it is the first in the
     *     queue. It returns null if the item is not found in the cache or is already processed recently.
     */
    public synchronized SequencerResponse<T> checkNext(final SequencerRequest<T> request) {
        Objects.requireNonNull(request, "request cannot be null");
        Objects.requireNonNull(request.resource(), "resource cannot be null");
        Objects.requireNonNull(request.resource().getMetadata(), "metadata cannot be null");

        final T resource = request.resource();
        final String namespace = resource.getMetadata().getNamespace();
        final String name = resource.getMetadata().getName();

        Sequenced sequencedItem = this.findOrRefresh(resource);
        if (sequencedItem == null) {
            List<Condition> conditions = List.of(new ConditionBuilder()
                    .withStatus("True")
                    .withType("Failed")
                    .withReason("SequencerError")
                    .withMessage("Resource '%s/%s' not found".formatted(namespace, name))
                    .build());
            final T desired = this.setStatus.apply(resource, FAILED, conditions);
            return new SequencerResponse<>(desired, Duration.ZERO, null, false);
        }

        final Duration reconcileDelay = computeReconcileDelay(request);
        final boolean isFirst = cache.isFirstInQueue(resource);
        final T desired = this.setStatus.apply(resource, PENDING, List.of());

        return new SequencerResponse<>(desired, reconcileDelay, sequencedItem, isFirst);
    }

    /**
     * Mark the resource as processed and perform any cleanup
     *
     * <p>Reconciler must call this after it has reconciled the resource successfully. It removes the resource from the
     * cache if it was the first in the queue.
     *
     * @param request the request including the resource and other metadata
     * @throws SequencerException if the item is not the first in the queue
     */
    public synchronized SequencerResponse<T> markAsProcessed(final SequencerRequest<T> request) {
        Objects.requireNonNull(request, "request cannot be null");
        Objects.requireNonNull(request.resource(), "resource cannot be null");

        final T resource = request.resource();

        final Sequenced first = cache.findFirst(resource.getMetadata().getNamespace());
        if (first == null) {
            throw new SequencerException("cache is empty; item cannot be removed from empty cache");
        }

        if (!first.uid().equals(resource.getMetadata().getUid())) {
            throw new SequencerException(
                    "resource is not the first item in the queue; it should not have been processed");
        }

        final Sequenced sequencedItem = cache.removeItem(resource);
        final Duration reconcileDelay = computeReconcileDelay(request);
        final T desired = this.setStatus.apply(resource, SUCCEEDED, List.of());
        return new SequencerResponse<>(desired, reconcileDelay, sequencedItem, false);
    }

    /**
     * Mark the resource as deleted and perform any cleanup
     *
     * <p>Reconciler must call this if it detects a resource is deleted. Metadata about the deleted resource is removed
     * from the cache.
     *
     * @param request the request including the resource and other metadata
     * @return true if the resource was found and removed from the cache
     */
    public synchronized boolean markAsDeleted(final SequencerRequest<T> request) {
        Objects.requireNonNull(request, "request cannot be null");
        Objects.requireNonNull(request.resource(), "resource cannot be null");

        final T resource = request.resource();
        final Sequenced item = cache.removeItem(resource);
        return item != null;
    }
}
