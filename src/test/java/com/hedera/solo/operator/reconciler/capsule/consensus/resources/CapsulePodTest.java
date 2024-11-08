// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaims;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.persistentvolumeclaims.Streams;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.ContainersBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.volumes.StreamsBuilder;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.test.utils.TemplateLoader;
import io.fabric8.kubernetes.api.model.EmptyDirVolumeSource;
import io.fabric8.kubernetes.api.model.HostPathVolumeSource;
import io.fabric8.kubernetes.api.model.HostPathVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpecBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSource;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.QuantityBuilder;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.api.model.VolumeMountBuilder;
import io.fabric8.kubernetes.api.model.VolumeResourceRequirementsBuilder;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import jdk.jfr.Description;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

public class CapsulePodTest {
    private final CapsulePod capsulePod = new CapsulePod();

    private final ConsensusCapsule consensusCapsule = Mockito.mock(ConsensusCapsule.class);
    private final ConsensusCapsuleSpec consensusCapsuleSpec = Mockito.mock(ConsensusCapsuleSpec.class);
    private final PersistentVolumeClaims persistentVolumeClaims = Mockito.mock(PersistentVolumeClaims.class);
    private final Streams streams = Mockito.mock(Streams.class);
    private final PodProperties podProperties = Mockito.mock(PodProperties.class);

    @BeforeEach
    void beforeEach() {
        Mockito.reset(consensusCapsule);
        Mockito.reset(consensusCapsuleSpec);
        Mockito.reset(persistentVolumeClaims);
        Mockito.reset(streams);
        Mockito.when(consensusCapsule.getSpec()).thenReturn(consensusCapsuleSpec);
        Mockito.when(persistentVolumeClaims.getStreams()).thenReturn(streams);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims()).thenReturn(persistentVolumeClaims);
        Mockito.when(consensusCapsule.getSpec().getPodProperties()).thenReturn(podProperties);
    }

    @Test
    @Description("test validation logic for PVC spec existence works")
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

    @Test
    @Description("test that stream uploader containers are prepared correctly")
    void testPrepareStreamUploaderContainers() {
        // no stream uploaders
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(new ContainersBuilder().build());
        Map<String, Optional<StreamUploader>> uploaderMap =
                capsulePod.prepareStreamUploaderContainers(consensusCapsule);
        Assertions.assertFalse(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).isPresent());
        Assertions.assertFalse(
                uploaderMap.get(Constants.TPL_VAR_EVENT_STREAM_UPLOADER).isPresent());
        Assertions.assertFalse(
                uploaderMap.get(Constants.TPL_VAR_BACKUP_UPLOADER).isPresent());
        Assertions.assertFalse(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .isPresent());

        // with valid stream uploaders settings it should return all stream uploaders
        StreamUploader rsu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_UPLOADER);
        StreamUploader esu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_EVENTS_STREAM_UPLOADER);
        StreamUploader bu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_BACKUP_UPLOADER);
        StreamUploader rssu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_SIDECAR_UPLOADER);
        rsu.setEnabled(true);
        esu.setEnabled(true);
        bu.setEnabled(true);
        rssu.setEnabled(true);
        Containers containers = new ContainersBuilder()
                .withRecordStreamUploader(rsu)
                .withRecordStreamSidecarFilesUploader(rssu)
                .withEventsStreamUploader(esu)
                .withBackupUploader(bu)
                .build();
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(containers);
        uploaderMap = capsulePod.prepareStreamUploaderContainers(consensusCapsule);
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_EVENT_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap.get(Constants.TPL_VAR_BACKUP_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .isPresent());

        // check if the rssu has signature config set to false
        Assertions.assertFalse(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .get()
                .getConfig()
                .getSignature()
                .getPrioritize());
        Assertions.assertFalse(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .get()
                .getConfig()
                .getSignature()
                .getRequire());

        // check if file extension set properly when compression is enabled
        rsu = uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).get();
        if (rsu.getConfig().getEnableCompression()) {
            Assertions.assertTrue(rsu.getConfig().getStreamExtension().endsWith(".gz"));
        } else {
            Assertions.assertFalse(rsu.getConfig().getStreamExtension().endsWith(".gz"));
        }
    }

    @Test
    @Description("test that stream uploader containers are skipped when disabled")
    void testPrepareStreamUploaderContainersWhenDisabled() {
        StreamUploader rsu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_UPLOADER);
        StreamUploader esu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_EVENTS_STREAM_UPLOADER);
        StreamUploader bu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_BACKUP_UPLOADER);
        StreamUploader rssu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_SIDECAR_UPLOADER);

        // disable rsu
        rsu.setEnabled(false);
        esu.setEnabled(true);
        bu.setEnabled(true);
        rssu.setEnabled(true);
        Containers containers = new ContainersBuilder()
                .withRecordStreamUploader(rsu)
                .withRecordStreamSidecarFilesUploader(rssu)
                .withEventsStreamUploader(esu)
                .withBackupUploader(bu)
                .build();
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(containers);
        Map<String, Optional<StreamUploader>> uploaderMap =
                capsulePod.prepareStreamUploaderContainers(consensusCapsule);
        Assertions.assertFalse(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_EVENT_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap.get(Constants.TPL_VAR_BACKUP_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .isPresent());

        // disable esu
        rsu.setEnabled(true);
        esu.setEnabled(false);
        bu.setEnabled(true);
        rssu.setEnabled(true);
        containers = new ContainersBuilder()
                .withRecordStreamUploader(rsu)
                .withRecordStreamSidecarFilesUploader(rssu)
                .withEventsStreamUploader(esu)
                .withBackupUploader(bu)
                .build();
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(containers);
        uploaderMap = capsulePod.prepareStreamUploaderContainers(consensusCapsule);
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).isPresent());
        Assertions.assertFalse(
                uploaderMap.get(Constants.TPL_VAR_EVENT_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap.get(Constants.TPL_VAR_BACKUP_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .isPresent());

        // disable bu
        rsu.setEnabled(true);
        esu.setEnabled(true);
        bu.setEnabled(false);
        rssu.setEnabled(true);
        containers = new ContainersBuilder()
                .withRecordStreamUploader(rsu)
                .withRecordStreamSidecarFilesUploader(rssu)
                .withEventsStreamUploader(esu)
                .withBackupUploader(bu)
                .build();
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(containers);
        uploaderMap = capsulePod.prepareStreamUploaderContainers(consensusCapsule);
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_EVENT_STREAM_UPLOADER).isPresent());
        Assertions.assertFalse(
                uploaderMap.get(Constants.TPL_VAR_BACKUP_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .isPresent());

        // disable rssu
        rsu.setEnabled(true);
        esu.setEnabled(true);
        bu.setEnabled(true);
        rssu.setEnabled(false);
        containers = new ContainersBuilder()
                .withRecordStreamUploader(rsu)
                .withRecordStreamSidecarFilesUploader(rssu)
                .withEventsStreamUploader(esu)
                .withBackupUploader(bu)
                .build();
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(containers);
        uploaderMap = capsulePod.prepareStreamUploaderContainers(consensusCapsule);
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_EVENT_STREAM_UPLOADER).isPresent());
        Assertions.assertTrue(uploaderMap.get(Constants.TPL_VAR_BACKUP_UPLOADER).isPresent());
        Assertions.assertFalse(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER)
                .isPresent());
    }

    @Test
    @Description("test additional volume mounts for stream uploader containers is set correctly")
    void testPrepareStreamUploaderContainersWithAdditionalVolumeMounts() {
        final VolumeMount tvm1 = new VolumeMountBuilder()
                .withName("test")
                .withMountPath("/test")
                .withSubPath("test.txt")
                .build();
        final VolumeMount tvm2 = new VolumeMountBuilder()
                .withName("test2")
                .withMountPath("/test")
                .withSubPath("test2.txt")
                .build();

        StreamUploader rsu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_UPLOADER);
        rsu.setEnabled(true);
        rsu.setAdditionalVolumeMounts(List.of(tvm1, tvm2));
        Containers containers =
                new ContainersBuilder().withRecordStreamUploader(rsu).build();
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(containers);

        Map<String, Optional<StreamUploader>> uploaderMap =
                capsulePod.prepareStreamUploaderContainers(consensusCapsule);
        Assertions.assertTrue(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).isPresent());
        Assertions.assertNotNull(
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).get().getAdditionalVolumeMounts());

        Assertions.assertEquals(
                2,
                uploaderMap
                        .get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER)
                        .get()
                        .getAdditionalVolumeMounts()
                        .size());

        // we should get back what we put in as volume mounts (for now we don't modify it, so it should come back as it
        // is)
        Assertions.assertTrue(uploaderMap
                .get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER)
                .get()
                .getAdditionalVolumeMounts()
                .contains(tvm1));
    }
}
