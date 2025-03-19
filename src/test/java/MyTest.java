import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import java.io.File;
import java.io.IOException;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.StreamSupport;
import org.junit.jupiter.api.Test;

public class MyTest {
/*
        var mapper = ObjectMapper(YAMLFactory())
        var f = project.layout.buildDirectory.dir("kubernetes").get().asFile.listFiles()
        for (ff in f!!) {
            var orig = project.layout.buildDirectory.dir("generated/object-model-generator/main/crd").get().file(ff.name).asFile
            var m1 = mapper.readValue(orig, Map::class.java)
            var m2 = mapper.readValue(ff, Map::class.java)
            println(ff.absolutePath)
        }
* */
    @Test
    public void test() throws IOException {
        ObjectMapper mapper = new ObjectMapper(new YAMLFactory());
        for (File file : new File("build/kubernetes").listFiles()) {
            if (!file.toString().contains("consensuscapsule")) {
                continue;
            }
            File orig = new File("build/generated/object-model-generator/main/crd/" + file.getName());
            if (orig.exists()) {
                JsonNode in = mapper.readTree(orig);
                JsonNode out = mapper.readTree(file);
                // update node, then write it back.
                update(map(in), map(out));
                int i = 1;
            }
        }
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
        int i = 1;
    }
}
