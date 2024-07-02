package com.hedera.solo.operator.test.framework.multimanytoone;

import static org.junit.jupiter.api.Assertions.assertEquals;

import com.hedera.solo.operator.model.v1alpha1.ApplicationState;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import com.hedera.solo.operator.test.framework.TestFramework;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import java.lang.invoke.MethodHandles;
import org.junit.jupiter.api.RepeatedTest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@TestFramework
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
            ConsensusCapsule cc = new ConsensusCapsuleBuilder().withNewMetadata().withName("node" + i).endMetadata().build();
            operator.create(cc);
//            Thread.sleep(100);
            NetworkInitialization networkInitialization = new NetworkInitializationBuilder().withNewMetadata().withName("ni" + i).endMetadata().build();
            operator.create(networkInitialization);
//            Thread.sleep(100);
        }
        Thread.sleep(5000);
        ApplicationState applicationState = operator.get(ApplicationState.class, "application-state");
        assertEquals(100, applicationState.getMetadata().getOwnerReferences().size());
        operator.delete(operator.get(ApplicationState.class, "application-state"));
//        applicationState.getMetadata().getOwnerReferences()
//                .stream()
//                .filter(x -> x.getKind().equals("NetworkInitialization"))
//                .forEach(o -> operator.delete(new NetworkInitializationBuilder().withNewMetadata().withName(o.getName()).endMetadata().build()));
        Thread.sleep(5000);
        applicationState = operator.get(ApplicationState.class, "application-state");
        assertEquals(100, applicationState.getMetadata().getOwnerReferences().size());
        log.info("cc: {}", operator.resources(ConsensusCapsule.class).list().getItems().stream().map(ResourceID::fromResource).toList());

//        // deploy network initialization
//        NetworkInitialization networkInitialization = new NetworkInitializationBuilder().withNewMetadata().withName("ni").endMetadata().build();
//        operator.create(networkInitialization);

//        while (b) {
//            Thread.sleep(1000);
//        }
//        log.info("errors: {} {}", ConsensusCapsuleReconciler.counter, NetworkInitializationReconciler.counter);
        int i = 1;
    }
    boolean b = true;
}
