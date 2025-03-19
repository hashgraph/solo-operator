import static io.fabric8.crdv2.generator.CRDGenerator.getOutputName;
import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import io.fabric8.crd.generator.collector.CustomResourceCollector;
import io.fabric8.crdv2.generator.CRDGenerationInfo;
import io.fabric8.crdv2.generator.CRDGenerator;
import io.fabric8.crdv2.generator.CRDPostProcessor;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.utils.ApiVersionUtil;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import java.util.stream.StreamSupport;
import org.junit.jupiter.api.Test;

public class MyCRD2GenTest {
    @Test
    public void test() {
        List<String> classPathElements = new ArrayList<>();
        List<File> filesToScan = new ArrayList<>();
        Path dir = Path.of("/var/home/devenv/dev/solo-operator/build/classes/java/main");
        try (Stream<Path> stream = Files.walk(dir)) {
            stream.filter(Files::isRegularFile)
                    .filter(path -> path.getFileName().toString().endsWith(".class") || path.getFileName().toString().endsWith(".jar"))
                    .map(Path::toFile).forEach(filesToScan::add);
        } catch (IOException e) {
            throw new RuntimeException(e);
        }

        CustomResourceCollector customResourceCollector = new CustomResourceCollector()
                .withClasspathElements(classPathElements)
                .withFilesToScan(filesToScan);
        List<Class<? extends HasMetadata>> customResourceClasses = customResourceCollector.findCustomResourceClasses();

        {
            new File("target/generated-sources/crd").mkdirs();
        }
        CRDGenerator crdGenerator = new CRDGenerator()
                .customResourceClasses(customResourceClasses)
                .inOutputDir(new File("target/generated-sources/crd"))
                .withPostProcessor(new MyPP());

        CRDGenerationInfo crdGenerationInfo = crdGenerator.detailedGenerate();
    }

    static class MyPP implements CRDPostProcessor {
        ObjectMapper objectMapper = new ObjectMapper(new YAMLFactory());
        KubernetesSerialization kubernetesSerialization = new KubernetesSerialization();
        Path inOutputDir = Path.of("/var/home/devenv/dev/solo-operator/build/generated/object-model-generator/main/crd");
        @Override
        public HasMetadata process(HasMetadata crd, String crdSpecVersion) {
            String version = ApiVersionUtil.trimVersion(crd.getApiVersion());
            String crdName = crd.getMetadata().getName();
            String outputName = getOutputName(crdName, version);
            File source = inOutputDir.resolve(outputName + ".yml").toFile();
            try {
                return kubernetesSerialization.unmarshal(process(objectMapper.readTree(source), objectMapper.readTree(kubernetesSerialization.asYaml(crd))));
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        }
        String process(JsonNode in, JsonNode out) throws JsonProcessingException {
            update(map(in), map(out));
            return objectMapper.writeValueAsString(out);
        }
        Map<String, JsonNode> map(JsonNode jn) {
            return StreamSupport.stream(jn.at("/spec/versions").spliterator(), false).collect(Collectors.toMap(e -> e.get("name").asText(), e -> e.at("/schema/openAPIV3Schema")));
        }

        void update(Map<String, JsonNode> in, Map<String, JsonNode> out) {
            for (String key : out.keySet()) {
                if (in.containsKey(key)) {
                    update(in.get(key), out.get(key));
                }
            }
        }
        void update(JsonNode in, JsonNode out) {
            for (Iterator<Map.Entry<String, JsonNode>> it = in.fields(); it.hasNext(); ) {
                var x = it.next();
                if (x.getValue() instanceof ObjectNode objectNode) {
                    if (!out.has(x.getKey()) && out instanceof ObjectNode o2) {
                        o2.put(x.getKey(), objectNode);
                        continue;
                    }
                }
                update(x.getValue(), out.get(x.getKey()));
            }
            int i = 1;
        }
    }
}
