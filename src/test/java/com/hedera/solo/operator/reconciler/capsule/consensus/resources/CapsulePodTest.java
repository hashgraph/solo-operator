// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaims;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.persistentvolumeclaims.Streams;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.volumes.StreamsBuilder;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.EmptyDirVolumeSource;
import io.fabric8.kubernetes.api.model.HostPathVolumeSource;
import io.fabric8.kubernetes.api.model.HostPathVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpecBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSource;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.QuantityBuilder;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.fabric8.kubernetes.api.model.VolumeResourceRequirementsBuilder;
import io.quarkus.test.junit.QuarkusTest;
import java.util.Map;
import java.util.Optional;
import jdk.jfr.Description;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

@QuarkusTest
public class CapsulePodTest {
    private final CapsulePod capsulePod = new CapsulePod();

    private final ConsensusCapsule consensusCapsule = Mockito.mock(ConsensusCapsule.class);
    private final ConsensusCapsuleSpec consensusCapsuleSpec = Mockito.mock(ConsensusCapsuleSpec.class);
    private final PersistentVolumeClaims persistentVolumeClaims = Mockito.mock(PersistentVolumeClaims.class);
    private final Streams streams = Mockito.mock(Streams.class);

    @BeforeEach
    void beforeEach() {
        Mockito.reset(consensusCapsule);
        Mockito.reset(consensusCapsuleSpec);
        Mockito.reset(persistentVolumeClaims);
        Mockito.reset(streams);
        Mockito.when(consensusCapsule.getSpec()).thenReturn(consensusCapsuleSpec);
        Mockito.when(persistentVolumeClaims.getStreams()).thenReturn(streams);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims()).thenReturn(persistentVolumeClaims);
    }

    @Test
    @Description("test validation logic for PVC spec existence")
    void testValidatePvcIsSpecified() {
        Assertions.assertThrows(
                IllegalArgumentException.class, () -> capsulePod.validatePvcIsSpecified(consensusCapsule, "INVALID"));

        // it would throw exception if desired PVC spec is not set
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(null);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getStates())
                .thenReturn(null);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getUpgrade())
                .thenReturn(null);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getStats())
                .thenReturn(null);
        Mockito.when(consensusCapsule
                        .getSpec()
                        .getPersistentVolumeClaims()
                        .getStreams()
                        .getRecord())
                .thenReturn(null);
        Mockito.when(consensusCapsule
                        .getSpec()
                        .getPersistentVolumeClaims()
                        .getStreams()
                        .getEvents())
                .thenReturn(null);
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_LOGS_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_STATES_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_STATS_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_UPGRADE_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX));

        // it would throw exception if PVC is null in the CapsuleSpec
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims()).thenReturn(null);
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_LOGS_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_STATES_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_STATS_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_UPGRADE_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.validatePvcIsSpecified(consensusCapsule, Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX));
    }

    @Test
    @Description("test that desired volume builder is returned for valid inputs")
    void testGetVolumeBuilder() {
        Volume inputVol = Mockito.mock(Volume.class);

        Assertions.assertThrows(
                NullPointerException.class,
                () -> capsulePod.getVolumeBuilder(null, inputVol, Constants.VOLUME_LOGS_NAME_SUFFIX));
        Assertions.assertThrows(
                NullPointerException.class,
                () -> capsulePod.getVolumeBuilder(consensusCapsule, null, Constants.VOLUME_LOGS_NAME_SUFFIX));
        Assertions.assertThrows(
                NullPointerException.class, () -> capsulePod.getVolumeBuilder(consensusCapsule, inputVol, null));
        Assertions.assertThrows(
                IllegalArgumentException.class,
                () -> capsulePod.getVolumeBuilder(consensusCapsule, inputVol, "INVALID"));

        Optional<VolumeBuilder> vb =
                capsulePod.getVolumeBuilder(consensusCapsule, inputVol, Constants.VOLUME_LOGS_NAME_SUFFIX);
        Assertions.assertTrue(vb.isEmpty());

        HostPathVolumeSource hpvs = new HostPathVolumeSourceBuilder()
                .withPath("/tmp/test")
                .withType("DirectoryOrCreate")
                .build();
        PersistentVolumeClaimVolumeSource pvcs = new PersistentVolumeClaimVolumeSourceBuilder()
                .withClaimName("node0-output")
                .build();

        // both hostpath and pvcs are not supported
        Mockito.when(inputVol.getHostPath()).thenReturn(hpvs);
        Mockito.when(inputVol.getPersistentVolumeClaim()).thenReturn(pvcs);
        Assertions.assertThrows(
                        IllegalArgumentException.class,
                        () -> capsulePod.getVolumeBuilder(
                                consensusCapsule, inputVol, Constants.VOLUME_LOGS_NAME_SUFFIX))
                .getMessage()
                .equals(CapsulePod.ERR_BOTH_HOSTPATH_PVC_NOT_ALLOWED);

        // check HostPath volume builder
        Mockito.when(inputVol.getHostPath()).thenReturn(hpvs);
        Mockito.when(inputVol.getPersistentVolumeClaim()).thenReturn(null);
        vb = capsulePod.getVolumeBuilder(consensusCapsule, inputVol, Constants.VOLUME_LOGS_NAME_SUFFIX);
        Assertions.assertTrue(vb.isPresent());
        Volume hv = vb.get().build();
        Assertions.assertEquals(hpvs.getPath(), hv.getHostPath().getPath());
        Assertions.assertEquals(hpvs.getType(), hv.getHostPath().getType());

        // check PVC builder
        Mockito.when(inputVol.getHostPath()).thenReturn(null);
        Mockito.when(inputVol.getPersistentVolumeClaim()).thenReturn(pvcs);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(new PersistentVolumeClaimSpecBuilder()
                        .withAccessModes("ReadWriteOnce")
                        .withResources(new VolumeResourceRequirementsBuilder()
                                .withRequests(Map.of(
                                        "storage",
                                        new QuantityBuilder().withAmount("50Mi").build()))
                                .build())
                        .build());
        vb = capsulePod.getVolumeBuilder(consensusCapsule, inputVol, Constants.VOLUME_LOGS_NAME_SUFFIX);
        Assertions.assertTrue(vb.isPresent());
        Volume pv = vb.get().build();
        Assertions.assertEquals(
                pvcs.getClaimName(), pv.getPersistentVolumeClaim().getClaimName());
    }

    @Test
    @Description("test that a list of desired list of pod volumes are returned")
    void testGetDesiredPodVolumes() {
        PodProperties pod = Mockito.mock(PodProperties.class);
        Volumes volumes = Mockito.mock(Volumes.class);
        Mockito.when(consensusCapsule.getSpec().getPodProperties()).thenReturn(pod);
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getVolumes()).thenReturn(volumes);

        // empty volumes
        Assertions.assertEquals(
                0, capsulePod.getDesiredPodVolumes(consensusCapsule).size());

        // with logs
        Mockito.when(volumes.getLogs())
                .thenReturn(new VolumeBuilder()
                        .withEmptyDir(new EmptyDirVolumeSource())
                        .build());
        Mockito.when(volumes.getStats())
                .thenReturn(new VolumeBuilder()
                        .withEmptyDir(new EmptyDirVolumeSource())
                        .build());
        Mockito.when(volumes.getStates())
                .thenReturn(new VolumeBuilder()
                        .withEmptyDir(new EmptyDirVolumeSource())
                        .build());
        Mockito.when(volumes.getUpgrade())
                .thenReturn(new VolumeBuilder()
                        .withEmptyDir(new EmptyDirVolumeSource())
                        .build());
        Mockito.when(volumes.getStreams())
                .thenReturn(new StreamsBuilder()
                        .withRecord(new VolumeBuilder()
                                .withEmptyDir(new EmptyDirVolumeSource())
                                .build())
                        .withEvents(new VolumeBuilder()
                                .withEmptyDir(new EmptyDirVolumeSource())
                                .build())
                        .build());
        Assertions.assertEquals(
                6, capsulePod.getDesiredPodVolumes(consensusCapsule).size());
    }
}
