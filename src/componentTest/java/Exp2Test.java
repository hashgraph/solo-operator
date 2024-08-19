import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.api.model.apps.DeploymentBuilder;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.lang.invoke.MethodHandles;
import java.util.Map;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class Exp2Test {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    KubernetesSerialization ks = new KubernetesSerialization();

    BuilderFunction<DeploymentBuilder> matchLabels(Map<String, String> labels) {
        return builder -> builder
                .editSpec()
                .editSelector()
                .addToMatchLabels(labels);
    }

    BuilderFunction<DeploymentBuilder> containerImage(String image) {
        return builder -> builder
                .editSpec()
                .editTemplate()
                .editSpec()
                .addNewContainer()
                .withImage(image);
    }

    BuilderFunction<DeploymentBuilder> metadata(HasMetadata primary, String name) {
        return builder -> builder
                .editMetadata()
                .withName(name)
                .withNamespace(primary.getMetadata().getNamespace());
    }

    @Test
    void test() {
        Deployment primary = edit(new Deployment())
                .with(b -> b
                        .editMetadata()
                        .withNamespace("namespace")
                        .withName("name"))
                .build();
        Deployment deployment = edit(new Deployment())
                .with(metadata(primary, "test"))
                .with(matchLabels(Map.of("match1", "label1")))
                .with(matchLabels(Map.of("match2", "label2")))
                .with(containerImage("image1"))
                .with(containerImage("image2"))
                .build();
        log.info("deployment: {}", ks.asYaml(deployment));
    }
}
