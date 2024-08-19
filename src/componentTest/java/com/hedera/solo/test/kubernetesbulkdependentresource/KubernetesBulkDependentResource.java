package com.hedera.solo.test.kubernetesbulkdependentresource;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Collectors;

@KubernetesDependent
public class KubernetesBulkDependentResource extends AbstractKubernetesBulkDependentResource<ConfigMap, Example> {
    private static final ObjectMapper mapper = new ObjectMapper(new YAMLFactory());
    private final boolean create;
    private final boolean delete;

    public KubernetesBulkDependentResource(boolean create, boolean delete) {
        super(ConfigMap.class);
        this.create = create;
        this.delete = delete;
    }

    @Override
    protected boolean create() {
        return create;
    }

    @Override
    protected boolean delete() {
        return delete;
    }

    List<String> targetList(Example primary) {
        try {
            return mapper.readValue(primary.getMetadata().getAnnotations().getOrDefault("target/list", "[]"), new TypeReference<List<String>>(){});
        } catch (JsonProcessingException ignored) {}
        return Collections.emptyList();
    }

    @Override
    public Map<String, ConfigMap> desiredResources(Example primary, Context<Example> context) {
        String namespace = primary.getMetadata().getNamespace();
        return toMap(targetList(primary).stream()
                .map(name -> edit(new ConfigMap())
                        .with(builder -> builder
                                .editMetadata()
                                .withName(name)
                                .withNamespace(namespace))
                        .with(builder -> builder
                                .addToData("updated", "true"))
                        .build()).toList());
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(Example primary) {
        String namespace = primary.getMetadata().getNamespace();
        return targetList(primary).stream()
                .map(name -> new ResourceID(name, namespace)).collect(Collectors.toSet());
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(ConfigMap resource) {
        return Set.of(new ResourceID(resource.getMetadata().getLabels().get("primary"), resource.getMetadata().getNamespace()));
    }
}
