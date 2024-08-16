package com.hedera.solo.test;

import static com.hedera.solo.test.ExpTest.Wrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import io.fabric8.kubernetes.api.builder.Builder;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.builder.Nested;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.lang.invoke.MethodHandles;
import java.util.Arrays;
import java.util.Map;
import java.util.function.Consumer;
import java.util.function.Function;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class ExpTest {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    KubernetesSerialization ks = new KubernetesSerialization();

    static class Wrapper<A, B extends Builder<A>> {
        private final B object;
        public Wrapper(B object) {
            this.object = object;
        }
//        public static <A, B extends Builder<A>> Wrapper<A, B> builder(Supplier<B> supplier) {
//            return new Wrapper<>(supplier.get());
//        }
        public static <A, B extends Builder<A>, C extends Editable<B>> Wrapper<A, B> edit(C object) {
            return new Wrapper<>(object.edit());
        }
//        public static <A, B extends Builder<A>> Wrapper<A, B> wrap(B object) {
//            return new Wrapper<>(object);
//        }
//        public static <A, B extends Nested<A>> Wrapper2<A, B> nested(Supplier<B> supplier) {
//            return new Wrapper2<>(supplier.get());
//        }
        public static <A, B extends Builder<A>, AA, BB extends Nested<AA>> Wrapper2<AA, BB, A, B> edit(BB object) {
            return new Wrapper2<>(object, null); // should not have ability to pop
        }
        public <AA, BB extends Nested<AA>> Wrapper2<AA, BB, A, B> editx(Function<B, BB> function) {
            return new Wrapper2<>(function.apply(object), this);
        }
        public <U> Wrapper<A, B> with(Function<B, U> function) {
            if (function.apply(object) instanceof Nested<?> nested) {
                for (Object obj = nested.and(); obj instanceof Nested<?>; obj = ((Nested<?>) obj).and()) {}
            }
            return this;
        }
        public <AA, BB extends Nested<AA>> Wrapper2<AA, BB, A, B> edit(Function<B, BB> function) {
            return new Wrapper2<>(function.apply(object), this);
        }
        public A build() {
            return object.build();
        }
//        public <R> R map(Function<T, R> function) {
//            return function.apply(object);
//        }
    }
    static class Wrapper2<A, B extends Nested<A>, AA, BB extends Builder<AA>> {
        private final B object;
        private final Wrapper<AA, BB> parent;
        public Wrapper2(B object, Wrapper<AA, BB> parent) {
            this.object = object;
            this.parent = parent;
        }
        public <U> Wrapper2<A, B, AA, BB> apply(Function<B, U> function) {
            if (function.apply(object) instanceof Nested<?> nested) {
                for (Object obj = nested.and(); obj instanceof Nested<?> && object != obj; obj = ((Nested<?>) obj).and()) {}
            }
            return this;
        }
        public Wrapper<AA, BB> pop() {
            return parent;
        }
    }


    <T> T apply(T object, Consumer<T>... consumer) {
        Arrays.stream(consumer).forEach(b -> b.accept(object));
        return object;
    }

    Object metadata(DeploymentBuilder builder) {
        return builder
                .editMetadata()
                .withName("name")
                .withNamespace("namespace");
    }

    Function<DeploymentBuilder, ?> metadata(String name, Deployment primary) {
        return builder -> builder
                .editMetadata()
                .withName(name)
                .withNamespace(primary.getMetadata().getNamespace());
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
        new OrbitBuilder().editSpec().editOrbitspecConsensus().and().and().build();
        Deployment primary = edit(new Deployment())
                .with(this::metadata)
                .with(b -> b
                        .editSpec()
                        .editTemplate()
                        .editSpec()
                        .addNewContainer()
                        .withImage("image"))
                .with(b -> b
                        .editMetadata()
                        .addToLabels("key", "value"))
                .with(b -> b
                        .editMetadata()
                        .addToLabels("key2", "label2"))
//                .nest(builder -> builder.editSpec())
                .editx(b -> b.editSpec())
//                .apply(builder -> edit(builder.editSpec())
                .apply(spec -> edit(spec.editSelector())
                        .apply(selector -> selector
                                .withMatchLabels(Map.of("match", "label"))))
                .pop()
                .build();
        log.info("asYaml: {}", ks.asYaml(primary));
        Deployment secondary = edit(new Deployment())
                .with(metadata("fred", primary))
                .build();
        log.info("asYaml: {}", ks.asYaml(secondary));
    }
}
