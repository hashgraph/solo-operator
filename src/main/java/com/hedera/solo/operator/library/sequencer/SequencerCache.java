// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashMap;
import java.util.LinkedList;
import java.util.List;
import java.util.Map;
import java.util.NoSuchElementException;
import java.util.Objects;

class SequencerCache<T extends HasMetadata> {
    // cache of items per namespace
    private final Map<String, List<Sequenced>> namespaceCache = new HashMap<>();

    // currently executing item per namespace
    private final Map<String, Sequenced> currentlyExecuting = new HashMap<>();

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

        // if already in the unprocessed cache, ignore the item
        if (findItem(resource) != null) {
            return false;
        }

        final List<Sequenced> items = namespaceCache.computeIfAbsent(namespace, (k) -> new LinkedList<>());
        items.add(Sequenced.from(resource));
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
            namespaceCache.get(namespace).remove(item);

            if (namespaceCache.get(namespace).isEmpty()) {
                namespaceCache.remove(namespace);
            }

            if (currentlyExecuting.get(namespace) != null
                    && currentlyExecuting
                            .get(namespace)
                            .uid()
                            .equals(resource.getMetadata().getUid())) {
                currentlyExecuting.remove(namespace);
            }

            return item;
        }

        return null;
    }

    Sequenced findFirst(String namespace) {
        if (!namespaceCache.containsKey(namespace)) {
            return null;
        }

        try {
            return namespaceCache.get(namespace).getFirst();
        } catch (NoSuchElementException e) {
            return null;
        }
    }

    boolean isFirstInQueue(T resource) {
        Sequenced first = findFirst(resource.getMetadata().getNamespace());

        // this solves the race condition where we have two resources crated at the same time but one is being processed
        // first
        final String namespace = resource.getMetadata().getNamespace();
        if (currentlyExecuting.get(namespace) != null
                && !currentlyExecuting
                        .get(namespace)
                        .uid()
                        .equals(resource.getMetadata().getUid())) {
            return false;
        }

        return first != null && first.uid().equals(resource.getMetadata().getUid());
    }

    Sequenced findLast(String namespace) {
        if (!namespaceCache.containsKey(namespace)) {
            return null;
        }

        try {
            return namespaceCache.get(namespace).getLast();
        } catch (NoSuchElementException e) {
            return null;
        }
    }

    Sequenced findItem(T resource) {
        final String namespace = resource.getMetadata().getNamespace();
        if (!namespaceCache.containsKey(namespace)) {
            return null;
        }

        for (Sequenced item : namespaceCache.get(namespace)) {
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

        List<Sequenced> items = namespaceCache.get(resource.getMetadata().getNamespace());
        if (items == null) {
            return -1;
        }
        return items.indexOf(findItem(resource));
    }

    boolean contains(T resource) {
        return findItem(resource) != null;
    }

    /**
     * Get all items in the cache for a given namespace in order
     *
     * @return an unmodifiable list of items
     */
    List<Sequenced> items(String namespace) {
        if (!namespaceCache.containsKey(namespace)) {
            return new ArrayList<>();
        }

        return Collections.unmodifiableList(namespaceCache.get(namespace));
    }

    long size(String namespace) {
        if (namespaceCache.containsKey(namespace)) {
            return namespaceCache.get(namespace).size();
        }

        return 0;
    }

    /** Remove cached items that are not active anymore */
    synchronized void prune(List<T> activeItems) {
        for (String namespace : namespaceCache.keySet()) {
            List<Sequenced> items = namespaceCache.get(namespace);

            // remove items that are not active anymore
            items.removeIf(item -> activeItems.stream()
                    .noneMatch(active -> active.getMetadata().getUid().equals(item.uid())));
        }

        // if currentlyExecuting is not active anymore, reset it
        for (String namespace : currentlyExecuting.keySet()) {
            if (activeItems.stream().noneMatch(active -> active.getMetadata()
                    .getUid()
                    .equals(currentlyExecuting.get(namespace).uid()))) {
                currentlyExecuting.remove(namespace);
            }
        }

        // remove empty namespace cache list
        namespaceCache.entrySet().removeIf(entry -> entry.getValue().isEmpty());
    }

    synchronized void markAsExecuting(T resource) {
        final String namespace = resource.getMetadata().getNamespace();
        if (currentlyExecuting.get(namespace) != null) {
            if (currentlyExecuting
                    .get(namespace)
                    .uid()
                    .equals(resource.getMetadata().getUid())) {
                return;
            }

            throw new SequencerException("Resource is already executing");
        }

        this.currentlyExecuting.put(namespace, Sequenced.from(resource));
    }

    synchronized Sequenced getCurrentlyExecuting(String namespace) {
        return this.currentlyExecuting.get(namespace);
    }

    /**
     * Get the sequence of items for a given namespace
     *
     * @param namespace the namespace
     * @return the sequence of item names
     */
    synchronized List<Sequenced> itemSequence(String namespace) {
        List<Sequenced> sequence = new ArrayList<>();
        if (namespaceCache.containsKey(namespace)) {
            for (Sequenced item : namespaceCache.get(namespace)) {
                sequence.add(Sequenced.clone(item));
            }
        }
        return sequence;
    }
}
