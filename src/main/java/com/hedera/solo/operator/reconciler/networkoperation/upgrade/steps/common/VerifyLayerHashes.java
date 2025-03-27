// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.Invalid;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.NotFound;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.Unknown;
import static com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus.Validity.Valid;

import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.library.container.ManifestResponse;
import com.hedera.solo.operator.library.container.oci.OCIImageIndex;
import com.hedera.solo.operator.library.container.oci.OCIImageManifest;
import com.hedera.solo.operator.library.data.AsciiArmor;
import com.hedera.solo.operator.model.commonspec.ImageRepository;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.commonspec.ImageVerificationSpec;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.lang.invoke.MethodHandles;
import java.util.Arrays;
import java.util.List;
import java.util.Objects;
import java.util.Random;
import java.util.function.BiConsumer;
import java.util.function.Function;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class VerifyLayerHashes
        extends VerifyLayerHashesAbstract<ImageVerificationSpec, ImageRepositoryStatus, ImageRepository> {
    private static final Logger log =
            LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    private final Random random = new Random();
    protected KubernetesClient kubernetesClient;
    protected String namespace;
    protected SoftwareVersionSource softwareVersionSource;

    /**
     * Main entry point to perform status update
     *
     * @param resource containing status to update
     * @param status to update
     * @param kubernetesClient a kubernetes client
     * @param softwareVersionSource contains the layer hashes to verify
     * @param conditionFunction provides a new condition
     * @param getStatusList provides the repository status list to update
     * @param setStatusList updates the repository status list
     * @return updated status
     */
    public static NetworkOperationStatus updateStatus(
            final HasMetadata resource,
            final NetworkOperationStatus status,
            final KubernetesClient kubernetesClient,
            final SoftwareVersionSource softwareVersionSource,
            final Function<NetworkOperationStatus, Condition> conditionFunction,
            final Function<SoftwareSourceStatus, List<ImageRepositoryStatus>> getStatusList,
            final BiConsumer<SoftwareSourceStatus, List<ImageRepositoryStatus>> setStatusList) {
        VerifyLayerHashes verifyLayerHashes =
                new VerifyLayerHashes(kubernetesClient, resource.getMetadata().getNamespace(), softwareVersionSource);
        return verifyLayerHashes.updateStatus(resource, status, conditionFunction, getStatusList, setStatusList);
    }

    /**
     * The selected image
     *
     * @param list of repository status
     * @return selected image string if one exists, otherwise null
     */
    public static String selectedImage(final List<ImageRepositoryStatus> list) {
        return image(selected(list));
    }

    /**
     * Constructor
     *
     * @see VerifyLayerHashes#updateStatus(HasMetadata, NetworkOperationStatus, KubernetesClient, SoftwareVersionSource,
     *     Function, Function, BiConsumer)
     */
    public VerifyLayerHashes(
            final KubernetesClient kubernetesClient,
            final String namespace,
            final SoftwareVersionSource softwareVersionSource) {
        this.kubernetesClient = kubernetesClient;
        this.namespace = namespace;
        this.softwareVersionSource = softwareVersionSource;
    }

    /**
     * Update status
     *
     * @see VerifyLayerHashes#updateStatus(HasMetadata, NetworkOperationStatus, KubernetesClient, SoftwareVersionSource,
     *     Function, Function, BiConsumer)
     */
    public NetworkOperationStatus updateStatus(
            final HasMetadata resource,
            final NetworkOperationStatus status,
            final Function<NetworkOperationStatus, Condition> conditionFunction,
            final Function<SoftwareSourceStatus, List<ImageRepositoryStatus>> getStatusList,
            final BiConsumer<SoftwareSourceStatus, List<ImageRepositoryStatus>> setStatusList) {
        NetworkOperationStatus result = status == null
                ? new NetworkOperationStatus()
                : edit(status).build(); // make a new copy instead of modifying the input
        if (result.getSoftwareSourceStatus() == null) {
            result.setSoftwareSourceStatus(new SoftwareSourceStatus());
        }
        SoftwareSourceStatus softwareSourceStatus = result.getSoftwareSourceStatus();
        setStatusList.accept(softwareSourceStatus, statusList(getStatusList.apply(softwareSourceStatus)));
        result.setConditions(conditions(resource, result.getConditions(), conditionFunction.apply(result)));
        return result;
    }

    @Override
    protected ImageCoordinates coordinates(final ImageRepository repository) {
        return coordinates(
                repository.getRepository() + "/" + repository.getImageName() + ":" + repository.getImageTag(),
                repository.getImagePullSecrets().stream()
                        .map(ref -> kubernetesClient
                                .secrets()
                                .inNamespace(namespace)
                                .withName(ref.getName())
                                .get())
                        .filter(Objects::nonNull)
                        .map(secret -> secret.getData().get(".dockerconfigjson"))
                        .filter(Objects::nonNull)
                        .map(AsciiArmor::base64Decode)
                        .toArray(String[]::new));
    }

    protected ImageCoordinates coordinates(final String image, final String... json) {
        return ImageCoordinates.https(image, json);
    }

    @Override
    protected ImageRepositoryStatus exception(final ImageRepository repository, final Exception e) {
        log.info("Exception in verifyLayerHashes", e);
        return imageRepositoryStatus(repository, null, Unknown);
    }

    protected ImageRepositoryStatus imageRepositoryStatus(
            final ImageRepository repository,
            final ImageVerificationSpec spec,
            final ImageRepositoryStatus.Validity validity) {
        return imageRepositoryStatus(repository, spec, validity, null);
    }

    protected ImageRepositoryStatus imageRepositoryStatus(
            final ImageRepository repository,
            final ImageVerificationSpec spec,
            final ImageRepositoryStatus.Validity validity,
            final String digest) {
        return edit(new ImageRepositoryStatus())
                .with(builder -> builder.withRepository(repository.getRepository())
                        .withImageName(repository.getImageName())
                        .withImageTag(repository.getImageTag())
                        .withValidity(validity)
                        .withManifestHash(digest)
                        .withImageVerificationSpec(spec)
                        .withImagePullSecrets(repository.getImagePullSecrets())
                        .withImagePullPolicy(ImageRepositoryStatus.ImagePullPolicy.valueOf(
                                softwareVersionSource.getImagePullPolicy().getValue())))
                .build();
    }

    @Override
    protected ImageRepositoryStatus invalid(final ImageRepository repository, final ManifestResponse response) {
        log.info("Invalid response for: {}", response.response.uri());
        return imageRepositoryStatus(repository, null, Unknown);
    }

    @Override
    protected ImageRepositoryStatus invalid(final ImageRepository repository, final String digest) {
        return imageRepositoryStatus(repository, null, digest == null ? Unknown : Invalid, digest);
    }

    @Override
    protected ImageRepositoryStatus statusCode(final ImageRepository repository, final int statusCode) {
        return imageRepositoryStatus(repository, null, statusCode == 404 ? NotFound : Unknown);
    }

    @Override
    protected ImageRepositoryStatus valid(
            final ImageRepository repository, final ImageVerificationSpec spec, final String digest) {
        return imageRepositoryStatus(repository, spec, Valid, digest);
    }

    @Override
    protected List<ImageRepository> repositories() {
        return softwareVersionSource.getImageRepositories();
    }

    protected List<ImageRepositoryStatus> statusList(final List<ImageRepositoryStatus> last) {
        List<ImageRepositoryStatus> list = super.statusList();
        ImageRepositoryStatus selected = selected(last);
        if (selected == null || !select(selected, list)) {
            List<ImageRepositoryStatus> valid = list.stream()
                    .filter(status -> Valid.equals(status.getValidity()))
                    .toList();
            if (!valid.isEmpty()) {
                valid.get(random.nextInt(valid.size())).setSelectedForDeployment(true);
            }
        }
        return list;
    }

    @Override
    protected List<ImageVerificationSpec> specs() {
        return softwareVersionSource.getImageVerificationSpec();
    }

    @Override
    protected boolean match(final ImageVerificationSpec spec, final OCIImageIndex.OCIImageIndexManifest manifest) {
        return (spec.getArchitecture() == null
                        || spec.getArchitecture().getValue().equals(manifest.platform.architecture))
                && (spec.getOs() == null || spec.getOs().getValue().equals(manifest.platform.os));
    }

    @Override
    protected boolean match(final ImageVerificationSpec spec, final OCIImageManifest manifest) {
        return Objects.equals(
                spec.getLayerHashes(),
                Arrays.stream(manifest.layers).map(layer -> layer.digest).toList());
    }

    protected boolean select(final ImageRepositoryStatus selected, final List<ImageRepositoryStatus> list) {
        ImageRepositoryStatus match = edit(selected)
                .with(builder -> builder.withSelectedForDeployment(false))
                .build();
        ImageRepositoryStatus found =
                list.stream().filter(match::equals).findAny().orElse(null);
        if (found != null) {
            found.setSelectedForDeployment(true);
            return true;
        }
        return false;
    }

    protected static ImageRepositoryStatus selected(final List<ImageRepositoryStatus> list) {
        return list == null
                ? null
                : list.stream()
                        .filter(ImageRepositoryStatus::getSelectedForDeployment)
                        .findAny()
                        .orElse(null);
    }

    protected static String image(final ImageRepositoryStatus status) {
        return status == null
                ? null
                : status.getRepository() + "/" + status.getImageName() + ":" + status.getManifestHash();
    }
}
