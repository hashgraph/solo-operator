package com.hedera.solo.test.kubernetesbulkdependentresource;

import static com.hedera.solo.operator.library.builder.BuilderHelper.edit;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.test.v1alpha1.Example;
import com.hedera.solo.operator.test.v1alpha1.ExampleBuilder;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.function.Consumer;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class UpdateOnlyBulkDependentResourceTest extends AbstractOperatorTest {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    protected LocallyRunOperatorExtension.Builder configure(LocallyRunOperatorExtension.Builder builder) {
        return builder.withReconciler(UpdateOnlyBulkDependentResourceReconciler.class);
    }

    @Test
    void test() {
        List<String> testCreate = List.of("test-create-1", "test-create-2");
        List<String> testNoCreate = List.of("test-no-create-1", "test-no-create-2");

        // create custom resource
        Example example = operator().create(edit(new Example())
                .with(builder -> builder
                        .editMetadata()
                        .withName("test-create"))
                .with(builder -> builder
                        .editSpec()
                        .withMap(Map.of(
                                ResourceCreate.class.getSimpleName(), String.join(",", testCreate),
                                ResourceNoCreate.class.getSimpleName(), String.join(",", testNoCreate)
                        )))
                .build());

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            for (String name : testCreate) {
                ConfigMap configMap = operator().get(ConfigMap.class, name);
                assertThat(configMap).isNotNull();
            }
            for (String name : testNoCreate) {
                ConfigMap configMap = operator().get(ConfigMap.class, name);
                assertThat(configMap).isNull();
            }
        });
    }

    @Test
    void testNoCreate() {
        List<String> names = List.of("test-no-create-1", "test-no-create-2");

        // create custom resource
        Example example = operator().create(edit(new Example())
                .with(builder -> builder
                        .editMetadata()
                        .withName("test-no-create"))
                .with(builder -> builder
                        .editSpec()
                        .withMap(Map.of(ResourceNoCreate.class.getSimpleName(), String.join(",", names))))
                .build());

        await().atLeast(Duration.ofSeconds(5)).untilTrue(new AtomicBoolean());

        for (String name : names) {
            ConfigMap configMap = operator().get(ConfigMap.class, name);
            assertThat(configMap).isNull();
        }
    }

    @Test
    void testx() {
        log.info("namespaces: {}", operator().getKubernetesClient().namespaces().list().getItems().stream().map(Namespace::getMetadata).map(ObjectMeta::getName).toList());

        // create configmaps
        for (int i = 0; i < 3; ++i) {
            final int j = i;
            operator().create(edit(new ConfigMap())
                    .with(builder -> builder
                            .editMetadata()
                            .withName("name" + j)
                            .withNamespace(operator().getNamespace()))
                    .build());
        }

        // create custom resource
        Example example = new ExampleBuilder()
                .editMetadata()
                .withName("example")
                .endMetadata()
                .editSpec()
                .withMessage("hello world!")
                .endSpec()
                .build();
        example = operator().create(example);

        // verify dependent resource was is created as expected
        await().atMost(Duration.ofSeconds(15)).untilAsserted(() -> {
            for (int i = 0; i < 3; ++i) {
                ConfigMap configMap = operator().get(ConfigMap.class, "name" + i);
                assertThat(configMap).hasFieldOrPropertyWithValue("metadata.name", "name" + i);
                assertThat(configMap).hasFieldOrPropertyWithValue("data.updated", "true");
            }
        });
    }
}
