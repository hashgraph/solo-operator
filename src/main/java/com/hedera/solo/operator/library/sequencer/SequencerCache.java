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

/**
 * SequencerCache is a thread-safe cache for managing Kubernetes resources that need to be processed in sequence. It
 * maintains a list of items per namespace, allowing for efficient addition, removal, and retrieval of items.
 *
 * @param <T> The type of Kubernetes resource that extends HasMetadata
 */
class SequencerCache<T extends HasMetadata> {
    // Cache of items per namespace
    private final Map<String, List<Sequenced>> namespaceCache = new HashMap<>();

    // Currently executing item per namespace
    private final Map<String, Sequenced> currentlyExecuting = new HashMap<>();

    /**
     * Adds an item to the cache if it does not already exist or if already processed.
     *
     * <p>It orders the items based on the creation time and then compares the name of the resource.
     *
     * <p>If the item exists in the unprocessed cache or the processed cache, it is ignored.
     *
     * @param resource The custom Kubernetes resource to be added.
     * @return true if the item is added to the unprocessed cache, false otherwise
     */
    synchronized boolean addItem(T resource) {
        Objects.requireNonNull(resource, "Resource cannot be null");
        Objects.requireNonNull(resource.getMetadata(), "Resource metadata cannot be null");
        Objects.requireNonNull(resource.getMetadata().getNamespace(), "Resource namespace cannot be null");
        Objects.requireNonNull(resource.getMetadata().getUid(), "Resource UID cannot be null");
        Objects.requireNonNull(resource.getMetadata().getCreationTimestamp(), "Resource creation time cannot be null");

        final String namespace = resource.getMetadata().getNamespace();

        // If the item already exists in the unprocessed cache, ignore it.
        if (findItem(resource) != null) {
            return false;
        }

        final List<Sequenced> items = namespaceCache.computeIfAbsent(namespace, (k) -> new LinkedList<>());
        items.add(Sequenced.from(resource));
        items.sort(Sequenced::compareTo);

        return true;
    }

    /**
     * Removes an item from the cache even if it is not the first item in the queue.
     *
     * @param resource The custom Kubernetes resource to be removed.
     * @return The removed Sequenced item, or null if the item was not found
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

    /**
     * Finds the first item in the queue for a given namespace.
     *
     * @param namespace The Kubernetes namespace.
     * @return The first Sequenced item, or null if no items are found
     */
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

    /**
     * Checks if the given resource is the first item in its queue.
     *
     * @param resource The custom Kubernetes resource to check.
     * @return true if the resource is the first item, false otherwise
     */
    boolean isFirstInQueue(T resource) {
        Sequenced first = findFirst(resource.getMetadata().getNamespace());

        // This solves the race condition where two resources are created at the same time but one is processed first.
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

    /**
     * Finds the last item in the queue for a given namespace.
     *
     * @param namespace The Kubernetes namespace.
     * @return The last Sequenced item, or null if no items are found
     */
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

    /**
     * Finds a specific item in the cache based on the resource.
     *
     * @param resource The custom Kubernetes resource to find.
     * @return The found Sequenced item, or null if not found
     */
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

    /**
     * Gets the index of a specific resource in its namespace's cache list.
     *
     * @param resource The custom Kubernetes resource to find the index for.
     * @return The index of the resource, or -1 if not found
     */
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

    /**
     * Checks if the cache contains a specific resource.
     *
     * @param resource The custom Kubernetes resource to check for existence.
     * @return true if the resource is in the cache, false otherwise
     */
    boolean contains(T resource) {
        return findItem(resource) != null;
    }

    /**
     * Gets all items in the cache for a given namespace in order.
     *
     * @param namespace The Kubernetes namespace.
     * @return An unmodifiable list of Sequenced items
     */
    List<Sequenced> items(String namespace) {
        if (!namespaceCache.containsKey(namespace)) {
            return new ArrayList<>();
        }

        return Collections.unmodifiableList(namespaceCache.get(namespace));
    }

    /**
     * Gets the size of the cache for a given namespace.
     *
     * @param namespace The Kubernetes namespace.
     * @return The number of items in the namespace cache
     */
    long size(String namespace) {
        if (namespaceCache.containsKey(namespace)) {
            return namespaceCache.get(namespace).size();
        }

        return 0;
    }

    /**
     * Prunes cached items that are not active anymore.
     *
     * @param activeItems The list of currently active items.
     */
    synchronized void prune(List<T> activeItems) {
        for (String namespace : namespaceCache.keySet()) {
            List<Sequenced> items = namespaceCache.get(namespace);

            // Remove items that are not active anymore.
            items.removeIf(item -> activeItems.stream()
                    .noneMatch(active -> active.getMetadata().getUid().equals(item.uid())));
        }

        // If currentlyExecuting is not active anymore, reset it.
        for (String namespace : currentlyExecuting.keySet()) {
            if (activeItems.stream().noneMatch(active -> active.getMetadata()
                    .getUid()
                    .equals(currentlyExecuting.get(namespace).uid()))) {
                currentlyExecuting.remove(namespace);
            }
        }

        // Remove empty namespace cache list entries.
        namespaceCache.entrySet().removeIf(entry -> entry.getValue().isEmpty());
    }

    /**
     * Marks a resource as being executed.
     *
     * @param resource The custom Kubernetes resource to mark as executing.
     * @throws SequencerException if the resource is already executing in the same namespace.
     */
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

    /**
     * Gets the currently executing resource for a given namespace.
     *
     * @param namespace The Kubernetes namespace.
     * @return The currently executing Sequenced item, or null if none is executing
     */
    synchronized Sequenced getCurrentlyExecuting(String namespace) {
        return this.currentlyExecuting.get(namespace);
    }

    /**
     * Gets the sequence of items for a given namespace in order.
     *
     * @param namespace The Kubernetes namespace.
     * @return The sequence of item names
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
