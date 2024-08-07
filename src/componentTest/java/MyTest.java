import static io.fabric8.kubernetes.client.Config.fromKubeconfig;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.dajudge.kindcontainer.ApiServerContainer;
import com.dajudge.kindcontainer.K3sContainer;
import com.dajudge.kindcontainer.KindContainer;
import com.dajudge.kindcontainer.KubernetesContainer;
import com.dajudge.kindcontainer.KubernetesWithKubeletContainer;
import com.hedera.solo.operator.reconciler.orbit.OrbitReconciler;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.NamespaceBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientBuilder;
import io.javaoperatorsdk.operator.junit.AbstractOperatorExtension;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Collections;
import org.eclipse.microprofile.config.ConfigProvider;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.testcontainers.junit.jupiter.Testcontainers;

@Testcontainers
public class MyTest {

    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
//    @Container
//    private static final KindContainer<?> container = new KindContainer<>();

    static KubernetesContainer<?> container;
    KubernetesContainer<?> container() {
        boolean enabled = ConfigProvider.getConfig().getOptionalValue("quarkus.kubernetes-client.devservices.enabled", Boolean.class).orElse(true);
        boolean override = ConfigProvider.getConfig().getOptionalValue("quarkus.kubernetes-client.devservices.override-kubeconfig", Boolean.class).orElse(false);
        String flavor = ConfigProvider.getConfig().getOptionalValue("quarkus.kubernetes-client.devservices.flavor", String.class).orElse("api-only");
        if (!enabled || !override) {
            return null;
        }
        if ("kind".equals(flavor)) {
            return new KindContainer<>();
        }
        if ("k3s".equals(flavor)) {
            KubernetesWithKubeletContainer<?> container = new K3sContainer<>();
            container.setBinds(Collections.emptyList());
            return container;
        }
        if ("api-only".equals(flavor)) {
            return new ApiServerContainer<>();
        }
        throw new IllegalArgumentException();
    }

    LocallyRunOperatorExtension.Builder operatorExtensionBuilder() {
        container = container();
        LocallyRunOperatorExtension.Builder builder = LocallyRunOperatorExtension.builder();
        if (null == container) {
            builder.withKubernetesClient(new KubernetesClientBuilder().build());
        } else {
            container.start();
            builder.withKubernetesClient(new KubernetesClientBuilder().withConfig(fromKubeconfig(container.getKubeconfig())).build());
        }
        return builder;
////                .withReconciler(new StatusUpdaterReconciler())
//                .withKubernetesClient(get())
//                .build();
    }

//    @Container
//    private K3sContainer<?> container2 = k3s();

    @RegisterExtension
    AbstractOperatorExtension operator = operatorExtensionBuilder()
            .withReconciler(new OrbitReconciler(org.jboss.logging.Logger.getLogger(OrbitReconciler.class)))
            .build();

    @AfterAll
    static void afterAll() {
        if (null != container) {
            container.stop();
        }
    }

    @Test
    void t2() {
        log.debug("debug");
        log.info("info");
        log.error("error");
    }

    @Test
    void test() {
//        assertThat(container).isNotNull();
//        assertThat(container2).isNotNull();

//        try (
        KubernetesClient client = operator.getKubernetesClient();
        Namespace ns = client.resource(new NamespaceBuilder().editMetadata().withName("test123").endMetadata().build()).create();
        log.error("namespaces: {}", client.namespaces().list().getItems().stream().map(x -> x.getMetadata().getName()).toList());
        client.resource(ns).delete();
        await().atMost(Duration.ofSeconds(60)).untilAsserted(() -> {
            assertThat(client.resource(ns).get()).isNull();
        });
        log.error("namespaces: {}", client.namespaces().list().getItems().stream().map(x -> x.getMetadata().getName()).toList());
//        }
//        try (KubernetesClient client = new KubernetesClientBuilder().withConfig(fromKubeconfig(container2.getKubeconfig())).build()) {
//            String kubeconfig = container2.getKubeconfig();
//            String config = Serialization.asYaml(client.getConfiguration());
//            Namespace ns = client.resource(new NamespaceBuilder().editMetadata().withName("test123").endMetadata().build()).create();
//            log.info("namespaces: {}", client.namespaces().list().getItems().stream().map(x -> x.getMetadata().getName()).toList());
//            client.resource(ns).delete();
//            await().atMost(Duration.ofSeconds(60)).untilAsserted(() -> {
//                assertThat(client.resource(ns).get()).isNull();
//            });
//            log.info("namespaces: {}", client.namespaces().list().getItems().stream().map(x -> x.getMetadata().getName()).toList());
//        }
//        try (KubernetesClient client = new KubernetesClientBuilder().build()) {
//            Namespace ns = client.resource(new NamespaceBuilder().editMetadata().withName("test123").endMetadata().build()).create();
//            log.info("namespaces: {}", client.namespaces().list().getItems().stream().map(x -> x.getMetadata().getName()).toList());
//            client.resource(ns).delete();
//            await().atMost(Duration.ofSeconds(60)).untilAsserted(() -> {
//                assertThat(client.resource(ns).get()).isNull();
//            });
//            log.info("namespaces: {}", client.namespaces().list().getItems().stream().map(x -> x.getMetadata().getName()).toList());
//        }
    }
}
