package com.hedera.solo.test.kubernetesbulkdependentresource;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.hedera.solo.operator.test.v1alpha1.Example;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;

/*
* Add test for update without create or delete!
* */
public class KubernetesBulkDependentResourceTest extends AbstractOperatorTest {
    private static final ObjectMapper mapper = new ObjectMapper(new YAMLFactory());

    @Override
    protected LocallyRunOperatorExtension.Builder configure(LocallyRunOperatorExtension.Builder builder) {
        KubernetesBulkDependentResource crud = new KubernetesBulkDependentResourceUpdater(true, true);
        KubernetesBulkDependentResource readOnly = new KubernetesBulkDependentResource(false, false);
        KubernetesBulkDependentResource updateOnly = new KubernetesBulkDependentResourceUpdater(false, false);
        crud.configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("primary=crud")
                .build());
        readOnly.configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("primary=read-only")
                .build());
        updateOnly.configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("primary=update-only")
                .build());
        return builder
                .withReconciler(new KubernetesBulkDependentResourceReconciler(crud), cfg -> cfg.withLabelSelector("crud").withName("crud"))
                .withReconciler(new KubernetesBulkDependentResourceReconciler(readOnly), cfg -> cfg.withLabelSelector("read-only").withName("read-only"))
                .withReconciler(new KubernetesBulkDependentResourceReconciler(updateOnly), cfg -> cfg.withLabelSelector("update-only").withName("update-only"));
    }

    record Config(String primary, String... names) {}

    @Test
    void test() {
        for (Config config : List.of(
                new Config("crud", "crud-update", "crud-delete"),
                new Config("read-only", "read-only-no-update", "read-only-no-delete"),
                new Config("update-only", "update-only-update", "update-only-no-delete"))) {
            for (String name: config.names) {
                operator().create(edit(new ConfigMap())
                        .with(builder -> builder
                                .editMetadata()
                                .addToLabels("primary", config.primary)
                                .withName(name))
                        .with(builder -> builder
                                .addToData("updated", "false"))
                        .build());
            }
        }

        List<Example> examples = new ArrayList<>();
        for (Config config : List.of(
                new Config("crud", "crud-create", "crud-update"),
                new Config("read-only", "read-only-no-create", "read-only-no-update"),
                new Config("update-only", "update-only-no-create", "update-only-update"))) {
            examples.add(operator().create(edit(new Example())
                    .with(builder -> builder
                            .editMetadata()
                            .withName(config.primary)
                            .addToLabels(config.primary, "true")
                            .addToAnnotations("target/list", mapper.writeValueAsString(config.names)))
                    .build()));
        }

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(operator().get(Example.class, "crud")).hasFieldOrPropertyWithValue("status.message", "2");
            assertThat(operator().get(Example.class, "read-only")).hasFieldOrPropertyWithValue("status.message", "1");
            assertThat(operator().get(Example.class, "update-only")).hasFieldOrPropertyWithValue("status.message", "1");
            for (String name : List.of("crud-create", "crud-update", "update-only-update")) {
                assertThat(operator().get(ConfigMap.class, name)).hasFieldOrPropertyWithValue("data.updated", "true");
            }
            for (String name : List.of("read-only-no-update", "read-only-no-delete", "update-only-no-delete")) {
                assertThat(operator().get(ConfigMap.class, name)).hasFieldOrPropertyWithValue("data.updated", "false");
            }
            for (String name : List.of("crud-delete", "read-only-no-create", "update-only-no-create")) {
                assertThat(operator().get(ConfigMap.class, name)).isNull();
            }
        });

        examples.forEach(operator()::delete);
    }
}
