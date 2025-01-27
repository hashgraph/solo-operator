// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.numeric.RolloverComparator;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Map;
import java.util.Objects;

/**
 * Sequencer ensures that the resources are processed in the correct sequence.
 *
 * <p>The reconciler should extend {@link AbstractSequenceReconciler} to operated on a custom resource that needs to be
 * reconciled in a correct sequence.
 *
 * <p>Sequencer maintains a config map with "head" and "tail" sequence numbers in the operator's namespace for each
 * reconciler and orbit combination. The "head" sequence number is the next sequence number to be processed and the
 * "tail" sequence number is the last item in teh queue.
 *
 * <p>The config name is derived from the reconciler class name and a suffix "sequencer-cm". For example, a config map
 * for {@link com.hedera.solo.operator.model.v1alpha1.NetworkOperation} resources would be
 * "networkoperation-sequencer-cm". Reconciler must invoke {@link Sequencer#load(CustomResource, Context, boolean)}
 * before using the sequencer.
 *
 * <p>The config map used by the sequencer is stored in the operator namespace, which detected from environment variable
 * KUBERNETES_NAMESPACE. If KUBERNETES_NAMESPACE is empty, then it defaults to "default" namespace.
 *
 * <p>Example config map of a sequencer:
 *
 * <pre>{@code
 * configmap:
 *     name: reconcilerX-sequencer-cm // <reconciler name in one word>-sequencer-cm
 * data:
 *     orbit1.head: 1 // namespace-1 head
 *     orbit1.tail: 2 // namespace-1 tail
 *     orbit2.head: 3 // namespace-2 head
 *     orbit2.tail: 5 // namespace-2 tail
 * }</pre>
 *
 * <p>The sequencer numbers (both head and tail numbers) in the queue starts with 0
 */
public class Sequencer<T extends CustomResource<S, R>, S extends KubernetesResource, R extends KubernetesResource> {
    private static final String HEAD_KEY_TEMPLATE = "%s.head"; // <namespace>.head
    private static final String TAIL_KEY_TEMPLATE = "%s.tail"; // <namespace>.tail

    private final RolloverComparator rolloverComparator = new RolloverComparator(63);
    private final String operatorNamespace;
    private final SequenceResourceManager<T, S, R> sequenceResourceManager;

    private ConfigMap cm;
    private final String configName;

    /**
     * Create a new sequencer
     *
     * @param sequenceResourceManager an object that implements {@link SequenceResourceManager} interface
     */
    public Sequencer(SequenceResourceManager<T, S, R> sequenceResourceManager) {
        this.sequenceResourceManager = sequenceResourceManager;

        // init
        this.configName = "%s-sequencer-cm".formatted(validNamePrefix(sequenceResourceManager.getName(), 40));
        this.operatorNamespace = System.getenv().getOrDefault("KUBERNETES_NAMESPACE", "default");
    }

    private String validNamePrefix(final String name, int max) {
        String finalName = name.toLowerCase().replaceAll("\\.", "-");
        if (finalName.length() > max) {
            finalName = finalName.substring(0, max);
        }
        return finalName;
    }

    /**
     * Load the config map of the sequencer
     *
     * <p>It is idempotent and can be called multiple times without any side effects.
     *
     * @param resource the custom resource that provides the namespace and uses the sequence number
     * @param context the context that provides the Kubernetes client
     * @param forced if true, it forces to reload the config map
     */
    public synchronized void load(final T resource, final Context<T> context, boolean forced) {
        // if we already have the config map, nothing to do
        if (!forced && Objects.nonNull(this.cm)) {
            return;
        }

        validateInputs(resource, context);

        final String namespace = resource.getMetadata().getNamespace();
        final String headKey = HEAD_KEY_TEMPLATE.formatted(namespace);
        final String tailKey = TAIL_KEY_TEMPLATE.formatted(namespace);
        final String orbitName = this.sequenceResourceManager.getOrbitName(resource);

        // prepare the initial ConfigMap
        ConfigMap initCm = edit(new ConfigMap())
                .with(builder -> Metadata.applyStandardLabels(
                                builder.editMetadata(),
                                configName,
                                orbitName,
                                Constants.COMPONENT_NET_OP,
                                Constants.COMPONENT_TRIGGER,
                                null)
                        .withName(this.configName)
                        .withNamespace(operatorNamespace))
                .with(data -> data.addToData(Map.of(
                        headKey, "0",
                        tailKey, "0")))
                .build();

        this.cm = context.getClient().resource(initCm).get();
        if (this.cm == null) {
            this.cm = context.getClient().resource(initCm).create();
        }
    }

    /**
     * Get the head sequence number for a given namespace
     *
     * @param namespace the namespace name
     * @return head sequence number
     */
    public Long headSequenceNumber(String namespace) {
        final String headKey = HEAD_KEY_TEMPLATE.formatted(namespace);
        if (cm.getData().get(headKey) == null) {
            throw new IllegalStateException(
                    "Head sequence number must not be null for '%s::%s".formatted(configName, headKey));
        }
        return cm.getData().get(headKey) != null ? Long.parseLong(cm.getData().get(headKey)) : 0;
    }

    public Long headSequenceNumber(T resource) {
        final String namespace = resource.getMetadata().getNamespace();
        return headSequenceNumber(namespace);
    }

    /**
     * Get the tail sequence number for a given namespace
     *
     * @param namespace the namespace name
     * @return tail sequence number
     */
    public Long tailSequenceNumber(String namespace) {
        final String tailKey = TAIL_KEY_TEMPLATE.formatted(namespace);
        if (cm.getData().get(tailKey) == null) {
            throw new IllegalStateException(
                    "Tail sequence number must not be null for '%s::%s".formatted(configName, tailKey));
        }
        return cm.getData().get(tailKey) != null ? Long.parseLong(cm.getData().get(tailKey)) : 0;
    }

    public Long tailSequenceNumber(T resource) {
        final String namespace = resource.getMetadata().getNamespace();
        return tailSequenceNumber(namespace);
    }

    /**
     * Update the config map with the new sequence number
     *
     * @param context the context
     * @param key the key to update
     * @param sequenceNumber the new sequence number
     * @throws IllegalStateException if the update fails
     */
    private void updateConfigMap(final Context<T> context, final String key, final long sequenceNumber) {
        cm.getData().put(key, String.valueOf(sequenceNumber));
        final ConfigMap updated = context.getClient().resource(cm).update();
        if (updated == null) {
            throw new IllegalStateException("failed to update config-map: '%s=%d'".formatted(key, sequenceNumber));
        }
        this.cm = updated;
    }

    /**
     * Push the resource in the queue with next sequence number
     *
     * <p>It also updates the config with new tail sequence number for next resource
     *
     * @param resource the custom resource
     * @return updated resource with sequence number
     */
    public synchronized T push(final T resource, final Context<T> context) {
        validateInputs(resource, context, cm);

        final long tail = tailSequenceNumber(resource);

        T updated = this.sequenceResourceManager.setSequenceNumber(resource, context, tail);

        final String tailKey =
                TAIL_KEY_TEMPLATE.formatted(resource.getMetadata().getNamespace());
        updateConfigMap(context, tailKey, rolloverComparator.rollover(tail + 1));

        return updated;
    }

    /**
     * Get the next sequence number that can be assigned to the resource
     *
     * <p>It also updates the config with new head sequence number for next processing
     *
     * @param resource the custom resource with the sequence number
     */
    public synchronized void pop(final T resource, final Context<T> context) {
        validateInputs(resource, context, cm);
        if (!isFirstInQueue(resource, context)) {
            throw new IllegalStateException("Resource is not first in the queue and cannot be popped");
        }

        final long head = headSequenceNumber(resource);
        final String headKey =
                HEAD_KEY_TEMPLATE.formatted(resource.getMetadata().getNamespace());
        updateConfigMap(context, headKey, rolloverComparator.rollover(head + 1));
    }

    /**
     * Check if the resource is first in the queue to be processed
     *
     * @param resource the custom resource
     * @return true if the resource is current, false otherwise
     */
    public synchronized boolean isFirstInQueue(final T resource, final Context<T> context) {
        validateInputs(resource, context, cm);

        final Long seq = this.sequenceResourceManager.getSequenceNumber(resource);
        if (seq == null || seq < -1) {
            return false;
        }

        return seq.equals(headSequenceNumber(resource));
    }

    private void validateInputs(final T resource, final Context<T> context, ConfigMap c) {
        validateInputs(resource, context);
        if (Objects.isNull(c)) {
            throw new IllegalStateException("ConfigMap must not be null; call load() first");
        }
    }

    private void validateInputs(final T resource, final Context<T> context) {
        Objects.requireNonNull(resource, "Resource must not be null");
        Objects.requireNonNull(context, "Context must not be null");
        Objects.requireNonNull(context.getClient(), "Kubernetes client must not be null");
    }
}
