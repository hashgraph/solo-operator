// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import org.jboss.logging.Logger;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.Objects;
import java.util.function.Supplier;

/**
 * Sequencer is responsible for ordering the processing of custom resources and computing the reconcile delay.
 *
 * @param <T> custom resource type
 */
public final class Sequencer<T extends HasMetadata> {
    private final Logger log = Logger.getLogger(Sequencer.class);

    private final SequencerCache<T> cache = new SequencerCache<>();
    private final Supplier<List<T>> activeResources;

    public static final Duration MAX_RECONCILIATION_DELAY = Duration.ofHours(1);
    public static final Duration DEFAULT_RECONCILIATION_DELAY = Duration.ofMillis(100);

    public static final Duration MAX_ESTIMATED_PROCESSING_TIME = Duration.ofHours(1);
    public static final Duration DEFAULT_ESTIMATED_PROCESSING_TIME = Duration.ofSeconds(1);

    // stores the time when the cache was refreshed last
    private Instant lastRefreshed;

    private static final Duration MAX_REFRESH_INTERVAL = Duration.ofMinutes(1);

    public Sequencer(final Supplier<List<T>> activeResources) {
        Objects.requireNonNull(activeResources, "activeResources parameter cannot be null");
        this.activeResources = activeResources;
    }

    /**
     * Refresh the cache using data from the Kubernetes API server using the provided supplier.
     */
    public synchronized void refresh() {
        List<T> activeItems = activeResources.get();
        if (activeItems == null || activeItems.isEmpty()) {
            return;
        }

        // remove old items from cache that are not active anymore
        cache.prune(activeItems);

        // add active items in the cache
        for (T item : activeItems) {
            cache.addItem(item);
        }

        lastRefreshed = Instant.now();
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

        // if last refreshed item is older than MAX_REFRESH_INTERVAL, force refresh the cache
        boolean refreshed = false;
        if (lastRefreshed == null || lastRefreshed.plus(MAX_REFRESH_INTERVAL).isBefore(Instant.now())) {
            this.refresh();
            refreshed = true;
        }

        Sequenced sequencedItem = cache.findItem(resource);
        if (sequencedItem != null || refreshed) {
            return sequencedItem;
        }

        // refresh and try again
        this.refresh();
        return cache.findItem(resource);
    }

    /**
     * Compute the reconcile delay based on the recommended delay, estimated processing time and the items in the queue.
     *
     * <p>Given a total of n items in the queue, it tries to estimate the processing time of all the earlier resources
     * in the queue and then selects 30% of the execution time as the reconciliation delay. This value is then compared
     * with the delay recommended by the reconciler and the larger value is selected.
     *
     * @param resource                custom resource
     * @param recommendedDelay        recommended delay
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
     * Check whether the resource is first in queue to be processed.
     *
     * <p>Reconciler must call this before it processes the resource.
     *
     * @param request the request including the resource and other metadata
     * @return the response including the resource, reconcile delay, sequenced item and whether it is the first in the
     * queue. It returns null if the item is not found in the cache or is already processed recently.
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
            return new SequencerResponse<>(Duration.ZERO, false, false);
        }

        final Duration reconcileDelay = computeReconcileDelay(request);
        final boolean isFirst = cache.isFirstInQueue(resource);

        return new SequencerResponse<>(reconcileDelay, isFirst, true);
    }

    /**
     * Mark the resource as processed and perform any cleanup
     *
     * <p>Reconciler must call this after it has reconciled the resource successfully. It removes the resource from the
     * cache if it was the first in the queue.
     *
     * @param request the request including the resource and other metadata
     * @return true if the resource was found to be the first and removed from the cache
     * @throws SequencerException if the resource is not the first in the queue
     */
    public synchronized boolean markAsCompleted(final SequencerRequest<T> request) {
        Objects.requireNonNull(request, "request cannot be null");
        Objects.requireNonNull(request.resource(), "resource cannot be null");

        final T resource = request.resource();

        final Sequenced first = cache.findFirst(resource.getMetadata().getNamespace());
        if (first == null) {
            log.warnf(
                    "resource '%s/%s' cannot be removed as the cache is empty",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());
            return false;
        }

        if (!first.uid().equals(resource.getMetadata().getUid())) {
            throw new SequencerException(
                    "resource '%s/%s' is not the first item in the queue; it should not have been processed"
                            .formatted(
                                    resource.getMetadata().getNamespace(),
                                    resource.getMetadata().getName()));
        }

        final Sequenced item = cache.removeItem(resource);
        return item != null;
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
