package com.hedera.solo.test.readonlybulkdependentresource;

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
import java.util.function.Consumer;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class ReadOnlyBulkDependentResourceTest extends AbstractOperatorTest {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    protected LocallyRunOperatorExtension.Builder configure(LocallyRunOperatorExtension.Builder builder) {
        return builder.withReconciler(ReadOnlyBulkDependentResourceReconciler.class);
    }

    <T> T x(T builder, Consumer<T>... consumer) {
        Arrays.stream(consumer).forEach(b -> b.accept(builder));
        return builder;
    }

    @Test
    void test() {
        log.info("namespaces: {}", operator().getKubernetesClient().namespaces().list().getItems().stream().map(Namespace::getMetadata).map(ObjectMeta::getName).toList());

        // crwate configmaps
        for (int i = 0; i < 3; ++i) {
            final int j = i;
            operator().create(
                    x(new ConfigMapBuilder(),
                            x -> x(x.editMetadata(),
                                    y -> y.withNamespace(operator().getNamespace())
                                            .withName("name" + j)
                            ).endMetadata()
                    ).build());
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

        int i = 1;
        // verify dependent resource was is created as expected
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConfigMap configMap = operator().get(ConfigMap.class, "example");
            assertThat(configMap).hasFieldOrPropertyWithValue("metadata.name", "example");
            assertThat(configMap).hasFieldOrPropertyWithValue("data.message", "hello world!");
        });
    }
}
