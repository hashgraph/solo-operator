package io.javaoperatorsdk.operator;

import java.time.Duration;
import java.util.HashMap;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;

import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import io.javaoperatorsdk.operator.sample.simple.TestCustomResource;
import io.javaoperatorsdk.operator.sample.simple.TestCustomResourceSpec;
import io.javaoperatorsdk.operator.sample.simple.TestReconciler;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

class ReconcilerExecutorTest {

  @RegisterExtension
  LocallyRunOperatorExtension operator =
      LocallyRunOperatorExtension.builder().withReconciler(new TestReconciler(true)).build();

  public static final String TEST_CUSTOM_RESOURCE_NAME = "test-custom-resource";

  TestCustomResource testCustomResource() {
    TestCustomResource resource = new TestCustomResource();
    String uid = UUID.randomUUID().toString();
    resource.setMetadata(
        new ObjectMetaBuilder()
            .withName(TEST_CUSTOM_RESOURCE_NAME)
            .withUid(uid)
            .withGeneration(1L)
            .build());
    resource.getMetadata().setAnnotations(new HashMap<>());
    resource.setKind("CustomService");
    resource.setSpec(new TestCustomResourceSpec());
    resource.getSpec().setConfigMapName("test-config-map");
    resource.getSpec().setKey("test-key");
    resource.getSpec().setValue("test-value");
    return resource;
  }

  @Test
  void configMapGetsCreatedForTestCustomResource() {
    operator.getReconcilerOfType(TestReconciler.class).setUpdateStatus(true);

    operator.create(testCustomResource());

    awaitResourcesCreatedOrUpdated();
    awaitStatusUpdated();
    // assertThat(TestUtils.getNumberOfExecutions(operator)).isEqualTo(2);
  }

  @Test
  void patchesStatusForTestCustomResource() {
    operator.getReconcilerOfType(TestReconciler.class).setPatchStatus(true);
    operator.getReconcilerOfType(TestReconciler.class).setUpdateStatus(true);

    TestCustomResource resource = testCustomResource();
    operator.create(resource);

    awaitStatusUpdated();
  }

  @Test
  void eventIsSkippedChangedOnMetadataOnlyUpdate() {
    operator.getReconcilerOfType(TestReconciler.class).setUpdateStatus(false);

    TestCustomResource resource = testCustomResource();
    operator.create(resource);

    awaitResourcesCreatedOrUpdated();
    // assertThat(TestUtils.getNumberOfExecutions(operator)).isEqualTo(1);
  }

  @Test
  void cleanupExecuted() {
    operator.getReconcilerOfType(TestReconciler.class).setUpdateStatus(true);

    TestCustomResource resource = testCustomResource();
    resource = operator.create(resource);

    awaitResourcesCreatedOrUpdated();
    awaitStatusUpdated();
    operator.delete(resource);

    await().atMost(Duration.ofSeconds(1))
        .until(() -> ((TestReconciler) operator.getFirstReconciler())
            .getNumberOfCleanupExecutions() == 1);
  }

  void awaitResourcesCreatedOrUpdated() {
    await("config map created")
        .atMost(5, TimeUnit.SECONDS)
        .untilAsserted(
            () -> {
              ConfigMap configMap =
                  operator.get(ConfigMap.class, "test-config-map");
              assertThat(configMap).isNotNull();
              assertThat(configMap.getData().get("test-key")).isEqualTo("test-value");
            });
  }

  void awaitStatusUpdated() {
    awaitStatusUpdated(5);
  }

  void awaitStatusUpdated(int timeout) {
    await("cr status updated")
        .atMost(timeout, TimeUnit.SECONDS)
        .untilAsserted(
            () -> {
              TestCustomResource cr =
                  operator.get(TestCustomResource.class,
                      TEST_CUSTOM_RESOURCE_NAME);
              assertThat(cr).isNotNull();
              assertThat(cr.getStatus()).isNotNull();
              assertThat(cr.getStatus().getConfigMapStatus()).isEqualTo("ConfigMap Ready");
            });
  }
}
