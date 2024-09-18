/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.reconciler.capsule.haproxy;

import static com.hedera.solo.operator.library.model.Labels.hasLabel;
import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY_KEY;
import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY_VALUE;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyConfigMap;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyConsensusSecret;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyConsensusService;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyDeployment;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyPod;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxySecret;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyService;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ControllerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceContext;
import io.javaoperatorsdk.operator.api.reconciler.EventSourceInitializer;
import io.javaoperatorsdk.operator.api.reconciler.Reconciler;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import io.javaoperatorsdk.operator.processing.dependent.workflow.Workflow;
import io.javaoperatorsdk.operator.processing.dependent.workflow.WorkflowBuilder;
import io.javaoperatorsdk.operator.processing.event.source.EventSource;
import java.lang.invoke.MethodHandles;
import java.util.Map;
import java.util.function.Predicate;
import org.jboss.logging.Logger;

@ControllerConfiguration
public class HAProxyCapsuleReconciler implements Reconciler<HAProxyCapsule>, EventSourceInitializer<HAProxyCapsule> {
    public static final Predicate<HasMetadata> HAS_LABEL_SELECTOR_HAPROXY =
            hasLabel(LABEL_SELECTOR_HAPROXY_KEY, LABEL_SELECTOR_HAPROXY_VALUE);
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private EventSourceContext<HAProxyCapsule> eventSourceContext;
    public final HAProxyConfigMap haProxyConfigMap = new HAProxyConfigMap();
    public final HAProxyConsensusSecret haProxyConsensusSecret = new HAProxyConsensusSecret(this);
    public final HAProxyConsensusService haProxyConsensusService = new HAProxyConsensusService(this);
    public final HAProxyDeployment haProxyDeployment = new HAProxyDeployment(this);
    public final HAProxyPod haProxyPod = new HAProxyPod();
    public final HAProxySecret haProxySecret = new HAProxySecret(this);
    public final HAProxyService haProxyService = new HAProxyService();
    private final Workflow<HAProxyCapsule> workflow = new WorkflowBuilder<HAProxyCapsule>()
            .addDependentResource(haProxyConfigMap)
            .addDependentResource(haProxyConsensusSecret)
            .addDependentResource(haProxyConsensusService)
            .addDependentResource(haProxyPod)
            .addDependentResource(haProxySecret)
            .addDependentResource(haProxyService)
            .addDependentResource(haProxyDeployment)
            .dependsOn(haProxyConfigMap, haProxySecret)
            .withReconcilePrecondition(new HAProxyDeployment.Ready(this))
            .build();

    public EventSourceContext<HAProxyCapsule> eventSourceContext() {
        return eventSourceContext;
    }

//    @Inject
//    Operator operator;
//    EventSource eventSource(Class<?> primary, Class<?> secondary) throws Exception {
//        Field field = ControllerHealthInfo.class.getDeclaredField("eventSourceManager");
//        field.setAccessible(true);
//        EventSourceManager<?> eventSourceManager = (EventSourceManager<?>) field.get(operator.getRegisteredControllers().stream()
//                .filter(x -> x.getConfiguration().getResourceClass().isAssignableFrom(primary))
//                .findAny().orElseThrow().getControllerHealthInfo());
//        return eventSourceManager.allEventSources().values().stream()
//                .filter(x -> x instanceof InformerEventSource<?,?>)
//                .map(x -> (InformerEventSource<?,?>)x)
//                .filter(x -> x.resourceType().isAssignableFrom(secondary))
//                .findAny().orElseThrow();
//    }
//
//    public Map<Class<?>, Map<String, EventSource>> allEventSources() throws Exception {
//        Field field = ControllerHealthInfo.class.getDeclaredField("eventSourceManager");
//        field.setAccessible(true);
//        return operator.getRegisteredControllers().stream()
//                .collect(Collectors.toMap(x -> x.getConfiguration().getResourceClass(), x -> {
//                    try {
//                        EventSourceManager<?> eventSourceManager = (EventSourceManager<?>) field.get(x.getControllerHealthInfo());
//                        return eventSourceManager.allEventSources();
//                    } catch (IllegalAccessException e) {
//                        throw new RuntimeException(e);
//                    }}));
//    }

    @Override
    public UpdateControl<HAProxyCapsule> reconcile(HAProxyCapsule resource, Context<HAProxyCapsule> context)
            throws Exception {
        try {
            log.infof(
                    "HAProxy Capsule - Starting Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
            workflow.reconcile(resource, context);

            HAProxyCapsuleStatus.Phase phase = HAProxyCapsuleStatus.Phase.Unknown;
            Pod[] pods = context.getSecondaryResources(Pod.class).toArray(Pod[]::new);
            if (1 == pods.length) {
                phase = HAProxyCapsuleStatus.Phase.valueOf(pods[0].getStatus().getPhase());
            }

            return UpdateControl.patchStatus(resource.edit()
                    .editStatus()
                    .withObservedGeneration(resource.getMetadata().getGeneration())
                    .withPhase(phase)
                    .endStatus()
                    .build());
        } finally {
            log.infof(
                    "HAProxy Capsule - Finished Reconciliation [ name = '%s' ]",
                    resource.getMetadata().getName());
        }
    }

    @Override
    public Map<String, EventSource> prepareEventSources(final EventSourceContext<HAProxyCapsule> context) {
        eventSourceContext = context;
        return EventSourceInitializer.eventSourcesFromWorkflow(context, workflow);
    }
}
