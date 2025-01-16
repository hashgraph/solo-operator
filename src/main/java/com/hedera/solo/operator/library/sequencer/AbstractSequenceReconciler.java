// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import java.time.Duration;
import java.util.Optional;

/**
 * AbstractSequenceReconciler is an abstract class that provides common functionalities for a reconciler to manage a
 * sequenced resource reconciliation
 *
 * <p>Subclass must override the abstract methods in order to be able to process the custom resource in sequence.
 *
 * <p>It is designed in a way so that any existing reconciler can extend this class and change the existing 'reconcile'
 * method to 'process' and be able to migrated to a sequenced reconciliation capability.
 *
 * @param <T> custom resource, e.g. {@link NetworkOperation}
 * @param <S> custom resource spec, e.g. {@link NetworkOperationSpec}
 * @param <R> custom resource status, e.g. {@link NetworkOperationStatus}
 */
public abstract class AbstractSequenceReconciler<
                T extends CustomResource<S, R>, S extends KubernetesResource, R extends KubernetesResource>
        implements Reconciler<T>, SequenceResourceManager<T, S, R> {

    protected final Sequencer<T, S, R> sequencer = new Sequencer<>(this);

    /**
     * A helper method to compute reconcile delay based on the resource spec and status
     *
     * <p>Here a concrete class can encapsulate the logic to specify different reconciliation delay as required
     *
     * @param resource a custom resource
     * @return reconcile delay duration
     */
    protected abstract Duration computeReconcileDelay(T resource);

    /**
     * It assigns a sequence number and additional metadata to the resource if not done already
     *
     * <p>It is a helper method to initialize the resource with sequence number and additional metadata if not done
     * already in the {@link AbstractSequenceReconciler#setSequenceNumber(CustomResource, Context, Long)} method.
     *
     * @param resource a custom resource that is to be sequenced
     * @param context a context object
     */
    protected Optional<T> initializeWithSequenceNumber(T resource, Context<T> context) {
        if (getSequenceNumber(resource) != null) {
            return Optional.empty();
        }

        T updated = sequencer.push(resource, context);
        return Optional.of(updated);
    }

    /**
     * Controller logic to initialize the resource with sequence number and additional metadata if not done already
     *
     * <p>Child class may not need to override it as {@link AbstractSequenceReconciler#initializeWithSequenceNumber} can
     * be overridden. However, this method is made protected to allow child class to override it if needed.
     *
     * <p>If child class need to override how a resource is initialized with the sequence number and additional
     * metadata, it needs to carefully check the following methods and override the logic accordingly:
     *
     * <ol>
     *   <li>{@link AbstractSequenceReconciler#setSequenceNumber(CustomResource, Context, Long)}: set sequence number,
     *       phase and metadata
     *   <li>{@link AbstractSequenceReconciler#initializeWithSequenceNumber(CustomResource, Context)}: auxiliary method
     *       to initialize the resource with additional metadata if not done already. Here child class may perform
     *       additional checks using context in order to set additional metadata or ignore if nothing to initialize.
     * </ol>
     *
     * @param resource NetworkOperation resource
     * @param context Context<NetworkOperation> context
     * @return UpdateControl<NetworkOperation> if it should skip processing it, null otherwise
     */
    protected UpdateControl<T> checkSequenceOrInitialize(T resource, Context<T> context) {
        Optional<T> initialized = initializeWithSequenceNumber(resource, context);
        if (initialized.isPresent()) {
            return UpdateControl.patchStatus(initialized.get())
                    .rescheduleAfter(computeReconcileDelay(initialized.get()));
        }

        return UpdateControl.<T>noUpdate().rescheduleAfter(computeReconcileDelay(resource));
    }

    /**
     * Child classes must override this method for reconciliation
     *
     * <p>This is synonymous to reconcile method of {@link Reconciler} interface
     *
     * @param resource custom resource
     * @param context context
     */
    protected abstract UpdateControl<T> process(T resource, Context<T> context) throws Exception;

    /**
     * Control logic for reconciliation
     *
     * <p>Child class shouldn't override this method, instead should implement 'process' method encapsulating the
     * reconciliation logic.
     *
     * <p>This method is a default implementation so that existing reconciler can easily migrate just by renaming
     * existing 'reconcile' method to 'process' and extending this class.
     *
     * @param resource custom resource
     * @param context context
     */
    public UpdateControl<T> reconcile(final T resource, final Context<T> context) throws Exception {
        sequencer.load(resource, context, false);

        if (!sequencer.isFirstInQueue(resource, context)) {
            return checkSequenceOrInitialize(resource, context);
        }

        final UpdateControl<T> resp = process(resource, context);
        if (resp.isPatchStatus()) {
            sequencer.pop(resource, context);
        }

        return resp;
    }
}
