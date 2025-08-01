// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.envoy;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.generic.Nullable.orNull;
import static com.hedera.solo.operator.library.reconcile.ReconcileEvent.cleanupEvents;
import static com.hedera.solo.operator.library.reconcile.ReconcileEvent.createOrUpdateEvent;
import static com.hedera.solo.operator.library.render.QuteUtils.generate;
import static com.hedera.solo.operator.library.render.QuteUtils.template;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import com.hedera.solo.operator.model.v1alpha1.EnvoyProxyBuilder;
import com.hedera.solo.operator.reconciler.envoy.resources.EnvoyConfigMap;
import com.hedera.solo.operator.reconciler.envoy.resources.EnvoyDeployment;
import com.hedera.solo.operator.reconciler.envoy.resources.EnvoyPodDisruptionBudget;
import com.hedera.solo.operator.reconciler.envoy.resources.EnvoyService;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentStatus;
import io.javaoperatorsdk.operator.api.reconciler.Cleaner;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.ErrorStatusUpdateControl;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceUtils;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import io.quarkiverse.operatorsdk.annotations.RBACRule;
import io.quarkus.qute.Template;
import java.util.List;

/** Reconciler for the EnvoyProxy resource. */
@RBACRule(
        apiGroups = "policy",
        resources = "poddisruptionbudgets",
        verbs = {"create", "delete", "get", "list", "patch", "update", "watch"})
public class EnvoyProxyReconciler implements Cleaner<EnvoyProxy>, Reconciler<EnvoyProxy> {
    Template template = template(getClass(), "envoy/envoy-proxy.yaml");
    EnvoyConfigMap envoyConfigMap = new EnvoyConfigMap();
    EnvoyDeployment envoyDeployment = new EnvoyDeployment();
    EnvoyPodDisruptionBudget envoyPodDisruptionBudget = new EnvoyPodDisruptionBudget();
    EnvoyService envoyService = new EnvoyService();
    Workflow<EnvoyProxy> workflow = new WorkflowBuilder<EnvoyProxy>()
            .addDependentResource(envoyConfigMap)
            .addDependentResource(envoyDeployment)
            .addDependentResource(envoyPodDisruptionBudget)
            .addDependentResource(envoyService)
            .build();

    @Override
    public UpdateControl<EnvoyProxy> reconcile(final EnvoyProxy resource, final Context<EnvoyProxy> context)
            throws Exception {
        workflow.reconcile(resource, context);
        return UpdateControl.patchStatus(edit(generate(EnvoyProxy.class, template, t -> t.data("primary", resource)))
                .with(status(resource, context))
                .build());
    }

    /**
     * Builds the status for the EnvoyProxy resource based on the associated Deployment status.
     *
     * @param resource The EnvoyProxy resource.
     * @param context The reconciliation context.
     * @return A BuilderFunction that updates the status of the EnvoyProxy resource
     */
    BuilderFunction<EnvoyProxyBuilder> status(final EnvoyProxy resource, final Context<EnvoyProxy> context) {
        Deployment deployment =
                envoyDeployment.getSecondaryResource(resource, context).orElse(null);
        DeploymentStatus deploymentStatus = orNull(() -> deployment.getStatus());
        return builder -> deploymentStatus == null
                ? builder
                : builder.editStatus()
                        .withReplicas(deploymentStatus.getReplicas())
                        .withAvailableReplicas(deploymentStatus.getAvailableReplicas())
                        .withReadyReplicas(deploymentStatus.getReadyReplicas())
                        .withTerminatingReplicas(deploymentStatus.getTerminatingReplicas())
                        .withUnavailableReplicas(deploymentStatus.getUnavailableReplicas())
                        .withUpdatedReplicas(deploymentStatus.getUpdatedReplicas())
                        .withConditions(deploymentStatus.getConditions());
    }

    @Override
    public List<EventSource<?, EnvoyProxy>> prepareEventSources(final EventSourceContext<EnvoyProxy> context) {
        return EventSourceUtils.eventSourcesFromWorkflow(context, workflow);
    }

    @Override
    public ErrorStatusUpdateControl<EnvoyProxy> updateErrorStatus(
            final EnvoyProxy resource, final Context<EnvoyProxy> context, final Exception e) {
        createOrUpdateEvent(resource, context, e);
        return ErrorStatusUpdateControl.noStatusUpdate();
    }

    @Override
    public DeleteControl cleanup(final EnvoyProxy resource, final Context<EnvoyProxy> context) throws Exception {
        cleanupEvents(resource, context);
        return DeleteControl.defaultDelete();
    }
}
