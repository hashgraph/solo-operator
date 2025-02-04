// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import com.hedera.solo.operator.model.commonspec.OpenTelemetryCollector;
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
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSource;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.api.model.VolumeMountBuilder;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
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
    @DisplayName("it should return persistent volume claim spec input based on volume name suffix")
    void testGetInputPvc() {
        Assertions.assertThrows(
                IllegalArgumentException.class, () -> capsulePod.getInputPvc(consensusCapsule, "INVALID"));

        // it would return empty if PVC is not specified in CapsuleSpec
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims()).thenReturn(null);
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_LOGS_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_STATES_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_STATS_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_UPGRADE_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX)
                .isPresent());

        // it would return empty if desired PVC spec is not set
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims()).thenReturn(persistentVolumeClaims);
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
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_LOGS_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_STATES_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_STATS_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_UPGRADE_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX)
                .isPresent());
        Assertions.assertFalse(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX)
                .isPresent());

        // it would return the input if desired PVC spec is set
        PersistentVolumeClaimSpec mockPvcSpec = Mockito.mock(PersistentVolumeClaimSpec.class);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(mockPvcSpec);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getStates())
                .thenReturn(mockPvcSpec);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getUpgrade())
                .thenReturn(mockPvcSpec);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getStats())
                .thenReturn(mockPvcSpec);
        Mockito.when(consensusCapsule
                        .getSpec()
                        .getPersistentVolumeClaims()
                        .getStreams()
                        .getRecord())
                .thenReturn(mockPvcSpec);
        Mockito.when(consensusCapsule
                        .getSpec()
                        .getPersistentVolumeClaims()
                        .getStreams()
                        .getEvents())
                .thenReturn(mockPvcSpec);

        Assertions.assertTrue(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_LOGS_NAME_SUFFIX)
                .isPresent());
        Assertions.assertTrue(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_STATES_NAME_SUFFIX)
                .isPresent());
        Assertions.assertTrue(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_STATS_NAME_SUFFIX)
                .isPresent());
        Assertions.assertTrue(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_UPGRADE_NAME_SUFFIX)
                .isPresent());
        Assertions.assertTrue(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX)
                .isPresent());
        Assertions.assertTrue(capsulePod
                .getInputPvc(consensusCapsule, Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX)
                .isPresent());
    }

    @Test
    @DisplayName("it should return volume builder for valid inputs")
    void testGetVolumeBuilder() {
        Volume inputVol = Mockito.mock(Volume.class);
        Assertions.assertThrows(
                NullPointerException.class,
                () -> capsulePod.getVolumeBuilder(null, inputVol, Constants.VOLUME_LOGS_NAME_SUFFIX));

        String volName = Constants.VOLUME_MOUNT_NAMES.get(Constants.VOLUME_LOGS_NAME_SUFFIX);
        Optional<VolumeBuilder> vb = capsulePod.getVolumeBuilder(consensusCapsule, inputVol, volName);
        Assertions.assertTrue(vb.isEmpty());

        HostPathVolumeSource hpvs = new HostPathVolumeSourceBuilder()
                .withPath("/tmp/test")
                .withType("DirectoryOrCreate")
                .build();
        PersistentVolumeClaimVolumeSource pvcs = new PersistentVolumeClaimVolumeSourceBuilder()
                .withClaimName("test-pvc")
                .build();

        // both hostpath and pvcs are not supported
        Mockito.when(inputVol.getHostPath()).thenReturn(hpvs);
        Mockito.when(inputVol.getPersistentVolumeClaim()).thenReturn(new PersistentVolumeClaimVolumeSource());
        Assertions.assertThrows(
                IllegalArgumentException.class, () -> capsulePod.getVolumeBuilder(consensusCapsule, inputVol, volName));

        // check HostPath volume
        Mockito.when(inputVol.getHostPath()).thenReturn(hpvs);
        Mockito.when(inputVol.getPersistentVolumeClaim()).thenReturn(null);
        vb = capsulePod.getVolumeBuilder(consensusCapsule, inputVol, volName);
        Assertions.assertTrue(vb.isPresent());
        Volume hv = vb.get().build();
        Assertions.assertEquals(hpvs.getPath(), hv.getHostPath().getPath());
        Assertions.assertEquals(hpvs.getType(), hv.getHostPath().getType());

        // check PVC volume
        Mockito.when(inputVol.getHostPath()).thenReturn(null);
        Mockito.when(inputVol.getPersistentVolumeClaim()).thenReturn(pvcs);
        vb = capsulePod.getVolumeBuilder(consensusCapsule, inputVol, volName);
        Assertions.assertTrue(vb.isPresent());
        Volume pv2 = vb.get().build();
        Assertions.assertEquals(
                pvcs.getClaimName(), pv2.getPersistentVolumeClaim().getClaimName());

        // check empty volume
        Mockito.when(inputVol.getEmptyDir()).thenReturn(new EmptyDirVolumeSource());
        Mockito.when(inputVol.getHostPath()).thenReturn(null);
        Mockito.when(inputVol.getPersistentVolumeClaim()).thenReturn(null);
        vb = capsulePod.getVolumeBuilder(consensusCapsule, inputVol, volName);
        Assertions.assertTrue(vb.isPresent());
        Assertions.assertNotNull(vb.get().build());
    }

    @Test
    @DisplayName("it should create volume based on specified PVC")
    void testGetPvcVolumeBuilder() {
        Volume inputVol = Mockito.mock(Volume.class);

        // if PVC or volumes are not specified, it should return empty
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(null);
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getVolumes()).thenReturn(null);
        Assertions.assertEquals(
                0, capsulePod.getDesiredPodVolumes(consensusCapsule).size());

        // return volume when PVC is specified
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(new PersistentVolumeClaimSpec());
        Assertions.assertEquals(
                1, capsulePod.getDesiredPodVolumes(consensusCapsule).size());
    }

    @Test
    @DisplayName("it should return a list of pod volumes")
    void testGetDesiredPodVolumes() {
        PodProperties pod = Mockito.mock(PodProperties.class);
        Volumes volumes = Mockito.mock(Volumes.class);
        Mockito.when(consensusCapsule.getSpec().getPodProperties()).thenReturn(pod);
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getVolumes()).thenReturn(volumes);

        // if a volume is duplicated in persistentVolumeClaims as well as volumes, it would throw error
        Mockito.when(volumes.getLogs())
                .thenReturn(new VolumeBuilder()
                        .withEmptyDir(new EmptyDirVolumeSource())
                        .build());
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(new PersistentVolumeClaimSpec());
        Assertions.assertThrows(
                IllegalArgumentException.class, () -> capsulePod.getDesiredPodVolumes(consensusCapsule));

        // reset and set empty volumes, it should return 0 volumes
        Mockito.when(volumes.getLogs()).thenReturn(null);
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(null);
        Assertions.assertEquals(
                0, capsulePod.getDesiredPodVolumes(consensusCapsule).size());

        // with volumes specified, it should return 6 volumes
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
    @DisplayName("it should return a list of persistent pod volumes")
    void testGetDesiredPvcVolumes() {
        // we just return mock PersistentVolumeClaimSpec since getDesiredPodVolumes does not use it
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getLogs())
                .thenReturn(new PersistentVolumeClaimSpec());
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getStats())
                .thenReturn(new PersistentVolumeClaimSpec());
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getStates())
                .thenReturn(new PersistentVolumeClaimSpec());
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getUpgrade())
                .thenReturn(new PersistentVolumeClaimSpec());
        Mockito.when(consensusCapsule.getSpec().getPersistentVolumeClaims().getStreams())
                .thenReturn(
                        new com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.persistentvolumeclaims
                                        .StreamsBuilder()
                                .withRecord(new PersistentVolumeClaimSpec())
                                .withEvents(new PersistentVolumeClaimSpec())
                                .build());
        Assertions.assertEquals(
                6, capsulePod.getDesiredPodVolumes(consensusCapsule).size());
    }

    @Test
    @DisplayName("it should return stream uploader containers")
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

        // unset stream file extensions to ensure it is set properly
        rsu.getConfig().setStreamExtension(null);
        rsu.getConfig().setStreamSigExtension(null);
        rsu.getConfig().setStreamExtension(null);
        rsu.getConfig().setStreamSigExtension(null);

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

        // check stream file extensions are set properly for record stream uploader
        StreamUploader rsu_returned =
                uploaderMap.get(Constants.TPL_VAR_RECORD_STREAM_UPLOADER).get();
        if (rsu_returned.getConfig().getEnableCompression()) {
            Assertions.assertTrue(rsu_returned.getConfig().getStreamExtension().endsWith(".gz"));
        } else {
            Assertions.assertFalse(rsu_returned.getConfig().getStreamExtension().endsWith(".gz"));
        }
        Assertions.assertEquals(
                Constants.RECORD_STREAM_EXTENSION, rsu_returned.getConfig().getStreamFileExtension());
        Assertions.assertEquals(
                Constants.RECORD_STREAM_SIG_EXTENSION, rsu_returned.getConfig().getStreamSigExtension());
        Assertions.assertEquals(
                Constants.RECORD_STREAM_SIG_EXTENSION, rsu_returned.getConfig().getSigExtension());

        // check stream file extensions are set properly for event stream uploaders
        StreamUploader esu_returned =
                uploaderMap.get(Constants.TPL_VAR_EVENT_STREAM_UPLOADER).get();
        Assertions.assertEquals(
                Constants.EVENT_STREAM_EXTENSION, esu_returned.getConfig().getStreamFileExtension());
        Assertions.assertEquals(
                Constants.EVENT_STREAM_EXTENSION, esu_returned.getConfig().getStreamFileExtension());
        Assertions.assertEquals(
                Constants.EVENT_STREAM_SIG_EXTENSION, esu_returned.getConfig().getStreamSigExtension());
        Assertions.assertEquals(
                Constants.EVENT_STREAM_SIG_EXTENSION, esu_returned.getConfig().getSigExtension());
    }

    @Test
    @DisplayName("it should not return stream uploader container when disabled")
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
    @DisplayName("it should set additional volume mounts for stream uploader containers")
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

    @Test
    @DisplayName("it should set additional volume mounts for otel-collector")
    void testPrepareOtelCollectorWithAdditionalVolumeMounts() {
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

        OpenTelemetryCollector oc =
                TemplateLoader.load(OpenTelemetryCollector.class, TemplateLoader.TEMPLATE_OPEN_TELEMETRY_COLLECTOR);
        oc.setEnabled(true);
        oc.setAdditionalVolumeMounts(List.of(tvm1, tvm2));
        Containers containers =
                new ContainersBuilder().withOpenTelemetryCollector(oc).build();
        Mockito.when(consensusCapsule.getSpec().getPodProperties().getContainers())
                .thenReturn(containers);

        Optional<OpenTelemetryCollector> opc = capsulePod.prepareOpenTelemetryCollector(consensusCapsule);
        Assertions.assertTrue(opc.isPresent());

        Assertions.assertEquals(2, opc.get().getAdditionalVolumeMounts().size());

        // we should get back what we put in as volume mounts (for now we don't modify it, so it should come back as it
        // is)
        Assertions.assertTrue(opc.get().getAdditionalVolumeMounts().contains(tvm1));
    }
}
