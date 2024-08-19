package com.hedera.solo.test.kubernetesbulkdependentresource;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.hedera.solo.operator.test.v1alpha1.Example;
import com.hedera.solo.operator.test.v1alpha1.ExampleBuilder;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class UpdateOnlyBulkDependentResourceTest extends AbstractOperatorTest {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    private static final ObjectMapper mapper = new ObjectMapper(new YAMLFactory());

    @Override
    protected LocallyRunOperatorExtension.Builder configure(LocallyRunOperatorExtension.Builder builder) {
        return builder.withReconciler(UpdateOnlyBulkDependentResourceReconciler.class);
    }

    String annotationKey(Class<?> aClass) {
        String string = aClass.getCanonicalName();
        int index = string.lastIndexOf('.');
        if (-1 != index) {
            string = string.substring(0, index) + "/" + string.substring(index + 1);
        }
        return string;
    }

    @Test
    void testxx() {
        List<String> testCreate = List.of("test-create-1", "test-create-2");
        List<String> testNoCreate = List.of("test-no-create-1", "test-no-create-2");

        KubernetesSerialization ks = new KubernetesSerialization();
        Example example = edit(new Example())
                .with(builder -> builder
                        .editMetadata()
                        .withName("test-create"))
//                .with(builder -> builder
//                        .editSpec()
//                        .withMap(Map.of(
//                                annotationKey(KubernetesBulkDependentResource.class), mapper.writeValueAsString(testCreate)//,
////                                annotationKey(ResourceNoCreate.class), mapper.writeValueAsString(testNoCreate)
//                        )))
                .with(builder -> builder
                        .editMetadata()
                        .withAnnotations(Map.of(
                                annotationKey(KubernetesBulkDependentResource.class), mapper.writeValueAsString(testCreate)//,
//                                annotationKey(ResourceNoCreate.class), mapper.writeValueAsString(testNoCreate)
                        )))
                .build();

        log.info("{}", ks.asYaml(example));
    }

    void createConfigMaps(List<String> names, Map<String, String> labels) {
        Stream.of(names).flatMap(List::stream).forEach(name -> {
            operator().create(edit(new ConfigMap())
                    .with(builder -> builder
                            .editMetadata()
                            .withName(name)
                            .addToLabels(labels)
                    )
                    .build());
        });
    }

    /*
    * This test is expected to produce logged exceptions.
    *
    * The BulkDependentResource implements the Creator interface by default, but here we test a
    * dependent resource which extends BulkDependentResource that intentionally fails to create.
    * */
    // simplify test to use single dr with either CRUD or read-only.
    @Test
    void test() {
        List<String> testCreate = List.of("test-create-1", "test-create-2");
        List<String> testNoCreate = List.of("test-no-create-1", "test-no-create-2");
        List<String> testDelete = List.of("test-delete-1", "test-delete-2");
        List<String> testNoDelete = List.of("test-no-delete-1", "test-no-delete-2");
        List<String> testUpdate = List.of("test-update-1", "test-update-2");
        List<String> testNoUpdate = List.of("test-no-update-1", "test-no-update-2");

        createConfigMaps(testDelete, Map.of("test-delete", "true"));
        createConfigMaps(testNoDelete, Collections.emptyMap());

        // create custom resource
        Example example = operator().create(edit(new Example())
                .with(builder -> builder
                        .editMetadata()
                        .withName("test-create"))
//                .with(builder -> builder
//                        .editSpec()
//                        .withMap(Map.of(
////                                ResourceCreate.class.getSimpleName(), mapper.writeValueAsString(testCreate),
////                                ResourceNoCreate.class.getSimpleName(), mapper.writeValueAsString(testNoCreate),
////                                ResourceDelete.class.getSimpleName(), mapper.writeValueAsString(testDelete)//,
////                                ResourceNoDelete.class.getSimpleName(), mapper.writeValueAsString(testNoDelete)
//                        )))
                .build());

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(Stream.of(testCreate/*, testNoDelete*/).flatMap(List::stream).map(name -> operator().get(ConfigMap.class, name))).allMatch(Objects::nonNull);
            assertThat(Stream.of(testNoCreate, testDelete).flatMap(List::stream).map(name -> operator().get(ConfigMap.class, name))).allMatch(Objects::isNull);
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
//                .with(builder -> builder
//                        .editSpec()
//                        .withMap(Map.of(ResourceNoCreate.class.getSimpleName(), String.join(",", values))))
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
