package com.hedera.solo.operator.test;

import com.dajudge.kindcontainer.KindContainer;
import com.hedera.solo.operator.test.framework.manytoone.StatusUpdaterReconciler;
import io.fabric8.kubernetes.api.model.DefaultKubernetesResourceList;
import io.fabric8.kubernetes.client.DefaultKubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientBuilder;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.testcontainers.junit.jupiter.Testcontainers;

@Disabled
@Testcontainers
public class NestedDemo {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    KindContainer<?> kind;

    KubernetesClient get() {
        kind = new KindContainer<>();
        kind.start();
        return new KubernetesClientBuilder().withConfig(kind.getKubeconfig()).build();
    }

    @RegisterExtension
    LocallyRunOperatorExtension operator = LocallyRunOperatorExtension.builder()
            .withReconciler(new StatusUpdaterReconciler())
            .withKubernetesClient(get())
            .build();

    @Test
    void test() {
        log.info("namespace: {}", operator.getNamespace());
    }
}
