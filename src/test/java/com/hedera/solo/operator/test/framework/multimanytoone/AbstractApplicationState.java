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

package com.hedera.solo.operator.test.framework.multimanytoone;

import com.hedera.solo.operator.library.dependentresources.ManyToOneKubernetesDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ApplicationState;
import com.hedera.solo.operator.model.v1alpha1.ApplicationStateBuilder;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.processing.event.ResourceID;

public abstract class AbstractApplicationState<P extends HasMetadata> extends ManyToOneKubernetesDependentResource<ApplicationState, P> {
    public AbstractApplicationState() {
        super(ApplicationState.class);
    }

    @Override
    protected ApplicationState createSecondaryResource(P primary) {
        return new ApplicationStateBuilder()
                .editOrNewMetadata()
                .withName("application-state")
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .build();
    }

    @Override
    protected ResourceID secondaryResourceID(P primary) {
        return new ResourceID("application-state", primary.getMetadata().getNamespace());
    }
}
