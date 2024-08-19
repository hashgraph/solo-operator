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
        crud.configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("primary=crud")
                .build());
        readOnly.configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("primary=read-only")
                .build());
        return builder
                .withReconciler(new KubernetesBulkDependentResourceReconciler(crud), cfg -> cfg.withLabelSelector("crud").withName("crud"))
                .withReconciler(new KubernetesBulkDependentResourceReconciler(readOnly), cfg -> cfg.withLabelSelector("read-only").withName("read-only"));
    }

    record KeyValues(String key, String... values) {}

    /*
     * This test is expected to produce logged exceptions (Created resource cannot be null).
     *
     * The BulkDependentResource implements the Creator interface by default, but here we test a
     * dependent resource which extends BulkDependentResource that intentionally fails to create.
     * */
    @Test
    void test() {
        for (KeyValues keyValues : List.of(
                new KeyValues("crud", "update", "delete"),
                new KeyValues("read-only", "no-update", "no-delete"))) {
            for (String value: keyValues.values) {
                operator().create(edit(new ConfigMap())
                        .with(builder -> builder
                                .editMetadata()
                                .addToLabels("primary", keyValues.key)
                                .withName(value))
                        .with(builder -> builder
                                .addToData("updated", "false"))
                        .build());
            }
        }
        for (KeyValues keyValues : List.of(
                new KeyValues("crud", "create", "update"),
                new KeyValues("read-only", "no-create", "no-update"))) {
            operator().create(edit(new Example())
                    .with(builder -> builder
                            .editMetadata()
                            .withName(keyValues.key)
                            .addToLabels(keyValues.key, "true")
                            .addToAnnotations("target/list", mapper.writeValueAsString(keyValues.values)))
                    .build());
        }

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            for (String name : List.of("create", "update")) {
                assertThat(operator().get(ConfigMap.class, name)).hasFieldOrPropertyWithValue("data.updated", "true");
            }
            for (String name : List.of("no-update", "no-delete")) {
                assertThat(operator().get(ConfigMap.class, name)).hasFieldOrPropertyWithValue("data.updated", "false");
            }
            assertThat(operator().get(ConfigMap.class, "no-create")).isNull();
        });
    }
}
