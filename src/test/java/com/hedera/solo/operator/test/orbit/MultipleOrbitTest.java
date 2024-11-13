package com.hedera.solo.operator.test.orbit;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.test.fixtures.Environment.isTestUsingGradleOrIntelliJ;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleFluent;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/*
⬢[devenv@toolbox com.hedera.solo.operator.test.capsule.ConsensusCapsuleTestAbstract#afterEach]$ yq '.data | keys' namespaced/consensuscapsuletest/configmaps/public-keys.yml
- a-public-node0.pem
- a-public-node1.pem
- a-public-node2.pem
- s-public-node0.pem
- s-public-node1.pem
- s-public-node2.pem
⬢[devenv@toolbox com.hedera.solo.operator.test.capsule.ConsensusCapsuleTestAbstract#afterEach]$ yq '.data | keys' namespaced/consensuscapsuletest/secrets/
mirror-secrets.yml       node0-grpc-tls-keys.yml  node1-grpc-tls-keys.yml  node2-grpc-tls-keys.yml
node0-gossip-keys.yml    node1-gossip-keys.yml    node2-gossip-keys.yml    uploader-secrets.yml
⬢[devenv@toolbox com.hedera.solo.operator.test.capsule.ConsensusCapsuleTestAbstract#afterEach]$ yq '.data | keys' namespaced/consensuscapsuletest/secrets/node0-gossip-keys.yml
- a-private-node0.pem
- s-private-node0.pem
⬢[devenv@toolbox com.hedera.solo.operator.test.capsule.ConsensusCapsuleTestAbstract#afterEach]$ yq '.data | keys' namespaced/consensuscapsuletest/secrets/node0-grpc-tls-keys.yml
- hedera-node0.crt
- hedera-node0.key

src/main/resources/templates/consensus-capsule/pod.yaml:    - name: gossip-keys
src/main/resources/templates/consensus-capsule/pod.yaml-      secret:
src/main/resources/templates/consensus-capsule/pod.yaml:        secretName: "{nodeId}-gossip-keys"
src/main/resources/templates/consensus-capsule/pod.yaml-        defaultMode: "{defaultMode}"

src/main/resources/templates/consensus-capsule/pod.yaml:    - name: grpc-tls-keys
src/main/resources/templates/consensus-capsule/pod.yaml-      secret:
src/main/resources/templates/consensus-capsule/pod.yaml:        secretName: "{nodeId}-grpc-tls-keys"
src/main/resources/templates/consensus-capsule/pod.yaml-        defaultMode: "{defaultMode}"

src/main/resources/templates/consensus-capsule/pod.yaml:    - name: public-keys
src/main/resources/templates/consensus-capsule/pod.yaml-      configMap:
src/main/resources/templates/consensus-capsule/pod.yaml:        name: public-keys
src/main/resources/templates/consensus-capsule/pod.yaml-        defaultMode: "{defaultMode}"

src/main/resources/templates/consensus-capsule/pod.yaml-            cp -rLfv /tmp/volumes/gossip-keys/* /opt/hgcapp/services-hedera/HapiApp2.0/data/keys/
src/main/resources/templates/consensus-capsule/pod.yaml:            cp -rLfv /tmp/volumes/public-keys/* /opt/hgcapp/services-hedera/HapiApp2.0/data/keys/

src/main/resources/templates/consensus-capsule/pod.yaml:            copyFile "$GRPC_TLS_VOL/hedera-{nodeId}.key" "$HAPI_APP/hedera.key"
src/main/resources/templates/consensus-capsule/pod.yaml:            copyFile "$GRPC_TLS_VOL/hedera-{nodeId}.crt" "$HAPI_APP/hedera.crt"

A       src/test/java/com/hedera/solo/operator/KeyTest.java
A       src/test/java/com/hedera/solo/operator/test/CertDownloadTest.java

* */

@QuarkusTest
@SoloTest
public class MultipleOrbitTest {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    @Inject
    protected Operator operator;

    List<Orbit> orbits = new ArrayList<>();

    @BeforeEach
    void setUp() {
        if (isTestUsingGradleOrIntelliJ()) {
            operator.start();
        }
        cleanup();
        Stream.of("alpha", "beta", "gamma")
                .map(name -> MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase() + "-" + name)
                .map(this::createOrbit).forEach(orbits::add);
    }

    @AfterEach
    void tearDown(final SnapshotFailureContext snapshot) {
        snapshot.client(operator.getKubernetesClient())
                .namespaces(orbits.stream().map(CustomResource::getMetadata).map(ObjectMeta::getName).toArray(String[]::new))
                .namespacedResources()
                .logs();
        cleanup();
    }

    void cleanup() {
        orbits.forEach(orbit -> operator.getKubernetesClient().resource(orbit).delete());
        await().atMost(Duration.ofSeconds(60)).untilAsserted(() -> {
            for (Orbit orbit : orbits) {
                assertThat(operator.getKubernetesClient().resource(orbit).get()).isNull();
                assertThat(operator.getKubernetesClient().namespaces().withName(orbit.getMetadata().getName()).get()).isNull();
            }
        });
    }

    @Test
    void test() {
        int[] id = new int[]{3};
        List<ConsensusCapsule> capsules = orbits.stream().map(o -> createCapsule(o, "node", id[0]++)).toList();
        List<NetworkInitialization> networkInitializations = orbits.stream().map(this::createNetworkInitializations).toList();

        await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
            for (NetworkInitialization ni : networkInitializations) {
                ni = operator.getKubernetesClient().resource(ni).get();
                assertThat(ni.getStatus()).isNotNull();
                assertThat(ni.getStatus().getPhase()).isNotNull();
                assertThat(ni.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Succeeded);
            }
        });
    }

    Orbit createOrbit(String name) {
        Orbit orbit = edit(new Orbit())
                .with(metadata -> metadata.editMetadata().withName(name))
                .with(spec -> spec.editSpec().editOrbitspecConsensus().editGenesis().editAddressBook().withLedgerId("298"))
                .build();
        return operator.getKubernetesClient().resource(orbit).create();
    }

    ConsensusCapsule createCapsule(Orbit orbit, String name, int accountId) {
        ConsensusCapsule capsule = edit(new ConsensusCapsule())
                .with(metadata -> metadata.editMetadata().withName(name)
                        .withNamespace(orbit.getMetadata().getName()))
                .with(spec -> spec.editSpec().withOrbit(orbit.getMetadata().getName())
                        .withAccountId("0.0." + accountId)
                        .editPod().editContainers().editConsensusNode()) // todo: <- why is this required at runtime?
                .build();
        return operator.getKubernetesClient().resource(capsule).create();
    }

    NetworkInitialization createNetworkInitializations(Orbit orbit) {
        NetworkInitialization networkInitialization = edit(new NetworkInitialization())
                .with(metadata -> metadata.editMetadata().withName("init")
                        .withNamespace(orbit.getMetadata().getName()))
                .build();
        return operator.getKubernetesClient().resource(networkInitialization).create();
    }
}
