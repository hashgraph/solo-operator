// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.Event;
import io.fabric8.kubernetes.api.model.EventBuilder;
import io.fabric8.kubernetes.api.model.MicroTime;
import io.fabric8.kubernetes.api.model.ObjectReferenceBuilder;
import java.time.Instant;
import java.util.Map;

public final class NetworkOperationUtil {
    public static Event triggerEvent(
            NetworkOperation resource, CustomResourceManager customResourceManager, Throwable ex) {
        final Instant now = Instant.now();
        final Event event = new EventBuilder()
                .withNewMetadata()
                .withName("NetworkOperationFailed")
                .withNamespace(resource.getMetadata().getNamespace())
                .withLabels(Map.of(Constants.LABEL_K8S_MANAGED_BY, Constants.SOLO_OPERATOR))
                .endMetadata()
                .withType("Error")
                .withReason("NetworkOperationFailed")
                .withMessage(ex.getMessage())
                .withEventTime(new MicroTime(now.toString()))
                .withFirstTimestamp(now.toString())
                .withReportingComponent("solo-operator")
                .withReportingInstance("solo-operator")
                .withAction("NetworkOperation")
                .withInvolvedObject(new ObjectReferenceBuilder()
                        .withName(resource.getMetadata().getName())
                        .withNamespace(resource.getMetadata().getNamespace())
                        .withUid(resource.getMetadata().getUid())
                        .withApiVersion(resource.getApiVersion())
                        .withKind(resource.getKind())
                        .withResourceVersion(resource.getMetadata().getResourceVersion())
                        .build())
                .build();

        return customResourceManager.triggerEvent(event);
    }
}
