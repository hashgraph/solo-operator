// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedList;
import java.util.List;
import java.util.Map;
import java.util.NoSuchElementException;
import java.util.Objects;

class SequencerCache<T extends HasMetadata> {
    private final Map<String, List<Sequenced>> unprocessed = new HashMap<>();

    // Maintain a cache of the recently processed items as Kubernetes client may return the same item multiple times.
    // This is because the status update of the resource takes time to be applied in a Kubernetes cluster.
    private final Map<String, Map<String, Sequenced>> processed = new HashMap<>();

    // Any item older than this value will be pruned from the processed cache
    private final Duration pruneAge;

    public SequencerCache() {
        pruneAge = Duration.ofMinutes(30);
    }

    public SequencerCache(Duration pruneAge) {
        this.pruneAge = pruneAge;
    }

    /**
     * Add the item to the cache if it does not already exist or if already processed
     *
     * <p>It orders the items based on the creation time and then compares the name of the resource.
     *
     * <p>If the item exists in the unprocessed cache or the processed cache, it is ignored.
     *
     * @param resource custom resource to be added
     * @return true if the item is added to the unprocessed cache, false otherwise
     */
    synchronized boolean addItem(T resource) {
        Objects.requireNonNull(resource, "Resource cannot be null");
        Objects.requireNonNull(resource.getMetadata(), "Resource metadata cannot be null");
        Objects.requireNonNull(resource.getMetadata().getNamespace(), "Resource namespace cannot be null");
        Objects.requireNonNull(resource.getMetadata().getUid(), "Resource UID cannot be null");
        Objects.requireNonNull(resource.getMetadata().getCreationTimestamp(), "Resource creation time cannot be null");

        final String namespace = resource.getMetadata().getNamespace();
        final String uid = resource.getMetadata().getUid();
        final String name = resource.getMetadata().getName();
        final Instant creationTime = Instant.parse(resource.getMetadata().getCreationTimestamp());

        // if recently processed, ignore the item
        if (processed.computeIfAbsent(namespace, (k) -> new HashMap<>()).containsKey(uid)) {
            return false;
        }

        // if already in the unprocessed cache, ignore the item
        if (findItem(resource) != null) {
            return false;
        }

        final List<Sequenced> items = unprocessed.computeIfAbsent(namespace, (k) -> new LinkedList<>());
        items.add(new Sequenced(namespace, uid, name, creationTime, Instant.now()));
        items.sort(Sequenced::compareTo);

        return true;
    }

    /**
     * Remove the item from the cache even if it is not the first item in the queue
     *
     * @param resource custom resource to be removed
     */
    synchronized Sequenced removeItem(T resource) {
        final String namespace = resource.getMetadata().getNamespace();
        final Sequenced item = findItem(resource);
        if (item != null) {
            unprocessed.get(namespace).remove(item);

            if (unprocessed.get(namespace).isEmpty()) {
                unprocessed.remove(namespace);
            }

            // put the item in the processed cache with new queuedAt time
            processed.computeIfAbsent(namespace, (k) -> new HashMap<>()).put(item.uid(), Sequenced.from(item));
            prune();

            return item;
        }

        return null;
    }

    Sequenced findFirst(String namespace) {
        if (!unprocessed.containsKey(namespace)) {
            return null;
        }

        try {
            return unprocessed.get(namespace).getFirst();
        } catch (NoSuchElementException e) {
            return null;
        }
    }

    boolean isFirstInQueue(T resource) {
        Sequenced first = findFirst(resource.getMetadata().getNamespace());
        return first != null && first.uid().equals(resource.getMetadata().getUid());
    }

    Sequenced findLast(String namespace) {
        if (!unprocessed.containsKey(namespace)) {
            return null;
        }

        try {
            return unprocessed.get(namespace).getLast();
        } catch (NoSuchElementException e) {
            return null;
        }
    }

    Sequenced findItem(T resource) {
        final String namespace = resource.getMetadata().getNamespace();
        if (!unprocessed.containsKey(namespace)) {
            return null;
        }

        for (Sequenced item : unprocessed.get(namespace)) {
            if (item.uid().equals(resource.getMetadata().getUid())) {
                return item;
            }
        }

        return null;
    }

    int indexOf(T resource) {
        if (resource == null) {
            return -1;
        }

        List<Sequenced> items = unprocessed.get(resource.getMetadata().getNamespace());
        if (items == null) {
            return -1;
        }
        return items.indexOf(findItem(resource));
    }

    boolean isProcessed(T resource) {
        return processed
                .computeIfAbsent(resource.getMetadata().getNamespace(), (k) -> new HashMap<>())
                .containsKey(resource.getMetadata().getUid());
    }

    /**
     * Get all items in the cache for a given namespace in order
     *
     * @return an unmodifiable list of items
     */
    List<Sequenced> items(String namespace) {
        if (!unprocessed.containsKey(namespace)) {
            return new ArrayList<>();
        }

        return Collections.unmodifiableList(unprocessed.get(namespace));
    }

    long size(String namespace) {
        if (unprocessed.containsKey(namespace)) {
            return unprocessed.get(namespace).size();
        }

        return 0;
    }

    /** Remove old items from the processed list */
    synchronized void prune() {
        final Instant now = Instant.now();

        processed.forEach((namespace, items) -> {
            items.entrySet().removeIf(entry -> entry.getValue()
                    .queuedAt()
                    .plusSeconds(pruneAge.toSeconds())
                    .isBefore(now));
        });

        // cleanup namespace cache if items are empty in the cache
        processed.entrySet().removeIf(entry -> entry.getValue().isEmpty());
    }
}
