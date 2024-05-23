package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.api.model.VolumeMountBuilder;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

@KubernetesDependent
public class CapsuleDeployment extends CRUDKubernetesDependentResource<Deployment, ConsensusCapsule> {

    public static final String TEMPLATE = "deployment.yaml";
    public static final String NAME_SUFFIX = "-deploy";

    public CapsuleDeployment() {
        super(Deployment.class);
    }

    @Override
    protected Deployment desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        ObjectMeta metadata = primary.getMetadata();
        String validName = validName(metadata.getName(), NAME_SUFFIX);

        boolean eligibleForDeployment = primary.getSpec() != null && primary.getSpec().getEligibleForDeployment();

        List<VolumeMount> volumeMounts = new ArrayList<>();
//        Set<String> names = new HashSet<>(context.getSecondaryResource(Secret.class).map(Secret::getData).map(Map::keySet).orElse(Collections.emptySet()));
//        names.removeAll(Set.of("hedera.crt", "hedera.key"));
        for (String name : Set.of("a", "s")) {
            String filename = name + "-private-" + metadata.getName() + ".pem";
            volumeMounts.add(new VolumeMountBuilder()
                    .withName("private-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + filename)
                    .withSubPath(filename)
                    .build());
        }
        for (String name : context.getClient().resources(ConfigMap.class).inNamespace(primary.getMetadata().getNamespace()).withName("public-keys").get().getData().keySet()) {
            volumeMounts.add(new VolumeMountBuilder()
                    .withName("public-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + name)
                    .withSubPath(name)
                    .build());
        }

        return loadYaml(Deployment.class, getClass(), TEMPLATE).edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .editSpec()
                .withReplicas(eligibleForDeployment ? 1 : 0)
                .editTemplate()
                .editMetadata()
                .addToLabels("name", metadata.getName())
                .endMetadata()
                .editSpec()
                .withHostname(metadata.getName())
                .editMatchingVolume(v -> "private-keys".equals(v.getName()))
                .editSecret()
                .withSecretName(validName(metadata.getName(), "-keys"))
                .endSecret()
                .endVolume()
                .editMatchingContainer(c -> "consensus-node".equals(c.getName()))
                .addAllToVolumeMounts(volumeMounts)
                .endContainer()
                .endSpec()
                .endTemplate()
                .endSpec()
                .build();
    }
}
