// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.reconcile.ReconcileEvent.cleanupEvents;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyConfigMap;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyDeployment;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxySecret;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyService;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.reconciler.Cleaner;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceUtils;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import io.quarkiverse.operatorsdk.annotations.RBACRule;
import java.lang.invoke.MethodHandles;
import java.util.List;
import org.jboss.logging.Logger;

/**
 * HAProxyCapsuleReconciler is responsible for reconciling the HAProxyCapsule resource. It manages the lifecycle of
 * HAProxy related resources.
 */
@ControllerConfiguration
@RBACRule(
        apiGroups = "",
        resources = "secrets",
        verbs = {"create", "delete", "get", "list", "patch", "update", "watch"})
@RBACRule(
        apiGroups = "",
        resources = "events",
        verbs = {"create", "delete", "get", "list", "patch", "update", "watch", "deletecollection"})
public class HAProxyCapsuleReconciler implements Cleaner<HAProxyCapsule>, Reconciler<HAProxyCapsule> {
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private EventSourceContext<HAProxyCapsule> eventSourceContext;
    public final HAProxyConfigMap haProxyConfigMap = new HAProxyConfigMap();
    public final HAProxyDeployment haProxyDeployment = new HAProxyDeployment(this);
    public final HAProxySecret haProxySecret = new HAProxySecret(this);
    public final HAProxyService haProxyService = new HAProxyService();
    private final Workflow<HAProxyCapsule> workflow = new WorkflowBuilder<HAProxyCapsule>()
            .addDependentResource(haProxyConfigMap)
            .addDependentResource(haProxySecret)
            .addDependentResource(haProxyService)
            .addDependentResourceAndConfigure(haProxyDeployment)
            .dependsOn(haProxyConfigMap, haProxySecret)
            .withReconcilePrecondition(new HAProxyDeployment.Ready(this))
            .build();

    /**
     * Returns the event source context for the HAProxyCapsule.
     *
     * @return {@code EventSourceContext<HAProxyCapsule>}
     */
    public EventSourceContext<HAProxyCapsule> eventSourceContext() {
        return eventSourceContext;
    }

    @Override
    public UpdateControl<HAProxyCapsule> reconcile(HAProxyCapsule resource, Context<HAProxyCapsule> context)
            throws Exception {
        try {
            log.infof(
                    "HAProxy Capsule - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
            workflow.reconcile(resource, context);

            Deployment deployment =
                    context.getSecondaryResource(Deployment.class).orElse(null);
            HAProxyCapsuleStatus.Phase phase = deployment == null
                            || deployment.getStatus() == null
                            || deployment.getStatus().getAvailableReplicas() == null
                            || deployment.getStatus().getAvailableReplicas() < 1
                    ? HAProxyCapsuleStatus.Phase.Unknown
                    : HAProxyCapsuleStatus.Phase.Running;

            return UpdateControl.patchStatus(edit(new HAProxyCapsule())
                    .with(metadata -> metadata.editMetadata()
                            .withName(resource.getMetadata().getName())
                            .withNamespace(resource.getMetadata().getNamespace()))
                    .with(status -> status.editStatus()
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withPhase(phase))
                    .build());
        } finally {
            log.infof(
                    "HAProxy Capsule - Finished Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
        }
    }

    @Override
    public List<EventSource<?, HAProxyCapsule>> prepareEventSources(final EventSourceContext<HAProxyCapsule> context) {
        eventSourceContext = context;
        return EventSourceUtils.eventSourcesFromWorkflow(context, workflow);
    }

    @Override
    public DeleteControl cleanup(HAProxyCapsule resource, Context<HAProxyCapsule> context) throws Exception {
        log.info("Deleting the resource: %s/%s "
                .formatted(
                        resource.getMetadata().getNamespace(),
                        resource.getMetadata().getName()));
        cleanupEvents(resource, context);
        return DeleteControl.defaultDelete();
    }
}
