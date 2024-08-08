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

import static org.junit.jupiter.api.Assertions.assertEquals;

import com.hedera.solo.operator.model.v1alpha1.ApplicationState;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.RepeatedTest;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@Disabled
//@TestFramework
public class ApplicationStateReconcilerTest {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @RegisterExtension
    LocallyRunOperatorExtension operator = LocallyRunOperatorExtension.builder()
            .withReconciler(new ApplicationStateReconciler())
            .withReconciler(new ConsensusCapsuleReconciler())
            .withReconciler(new NetworkInitializationReconciler())
            .build();

    @RepeatedTest(5)
    void test() throws Exception {
        // deploy capsules
        for (int i = 0; i < 50; ++i) {
            ConsensusCapsule cc = new ConsensusCapsuleBuilder()
                    .withNewMetadata()
                    .withName("node" + i)
                    .endMetadata()
                    .build();
            operator.create(cc);
            NetworkInitialization networkInitialization = new NetworkInitializationBuilder()
                    .withNewMetadata()
                    .withName("ni" + i)
                    .endMetadata()
                    .build();
            operator.create(networkInitialization);
        }
        Thread.sleep(5000);
        ApplicationState applicationState = operator.get(ApplicationState.class, "application-state");
        assertEquals(100, applicationState.getMetadata().getOwnerReferences().size());
        operator.delete(operator.get(ApplicationState.class, "application-state"));
        Thread.sleep(5000);
        applicationState = operator.get(ApplicationState.class, "application-state");
        assertEquals(100, applicationState.getMetadata().getOwnerReferences().size());
    }
}
