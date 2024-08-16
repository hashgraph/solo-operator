package com.hedera.solo.test;

import static com.hedera.solo.test.ExpTest.Wrapper.wrap;

import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
import io.fabric8.kubernetes.api.model.apps.DeploymentFluent;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.lang.invoke.MethodHandles;
import java.util.Arrays;
import java.util.function.Consumer;
import java.util.function.Function;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class ExpTest {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    KubernetesSerialization ks = new KubernetesSerialization();

    static class Wrapper<T> {
        T object;
        public Wrapper(T object) {
            this.object = object;
        }
        public static <T> Wrapper<T> wrap(T object) {
            return new Wrapper<>(object);
        }
        public Wrapper<T> apply(Consumer<T> consumer) {
            consumer.accept(object);
            return this;
        }
        public <R> R map(Function<T, R> function) {
            return function.apply(object);
        }
    }


    <T> T apply(T object, Consumer<T>... consumer) {
        Arrays.stream(consumer).forEach(b -> b.accept(object));
        return object;
    }

    void metadata(DeploymentBuilder builder) {
        builder
                .editMetadata()
                .withName("name")
                .withNamespace("namespace")
                .endMetadata();
    }

    Consumer<DeploymentBuilder> metadata(String name, Deployment primary) {
        return builder -> builder
                .editMetadata()
                .withName(name)
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata();
    }

    @Test
    void test() {
        Deployment deployment = apply(new DeploymentBuilder(), this::metadata).build();
        log.info("asYaml: {}", ks.asYaml(deployment));
        deployment = apply(deployment.edit(),
                b -> b
                        .editMetadata()
                        .addToLabels("key", "value")
                        .endMetadata())
                .build();
        log.info("asYaml: {}", ks.asYaml(deployment));
        deployment = apply(deployment.edit(),
                b -> apply(b.editMetadata()).endMetadata())
                .build();
    }

    @Test
    void test2() {
        Deployment primary = wrap(new DeploymentBuilder())
                .apply(this::metadata)
                .apply(b -> b
                        .editMetadata()
                        .addToLabels("key", "value")
                        .endMetadata())
                .apply(b -> wrap(b.editMetadata())
                        .apply(c -> c
                                .addToLabels("key2", "label2"))
                        .map(DeploymentFluent.MetadataNested::endMetadata))
                .map(DeploymentBuilder::build);
        log.info("asYaml: {}", ks.asYaml(primary));
        Deployment secondary = wrap(new DeploymentBuilder())
                .apply(metadata("fred", primary))
                .map(DeploymentBuilder::build);
        log.info("asYaml: {}", ks.asYaml(secondary));
    }
}
