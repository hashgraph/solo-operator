package com.hedera.solo.test.labelselector;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.test.v1alpha1.Example;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.time.Duration;
import org.junit.jupiter.api.Test;

public class LabelSelectorTest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(LocallyRunOperatorExtension.Builder builder) {
        return builder.withReconciler(LabelSelectorReconciler.class);
    }

/*
* I think the selector is filtering out events, I can verify that by leaving it run with debug logging enabled
* and manually generating events with and without labels
*
* we may want to use label selectors more often for performance reasons.
* */
    @Test
    void test() {
        operator().create(edit(new Example())
                .with(builder -> builder
                        .editMetadata()
                        .withName("label-selector-test")
                        .addToLabels("test-label-selector", "true")
                )
                .build());

        operator().create(edit(new ConfigMap())
                .with(builder -> builder
                        .editMetadata()
                        .withName("label-selector-test")
                        .addToLabels("test-label-selector", "true")
                )
                .build());

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(operator().get(ConfigMap.class, "label-selector-test")).hasFieldOrProperty("data.updated");
//            assertThat(operator().get(ConfigMap.class, "normal")).isNotNull();
        });

//        try {
//            Thread.sleep(90000);
//        } catch (InterruptedException e) {
//            throw new RuntimeException(e);
//        }
    }
}
