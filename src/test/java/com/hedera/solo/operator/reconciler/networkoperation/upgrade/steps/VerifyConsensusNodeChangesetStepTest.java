// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyConsensusNodeChangesetStep.FAIL_STATUS_MESSAGE;
import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.ImageRepository;
import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatusBuilder;
import com.hedera.solo.operator.test.utils.TemplateLoader;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

/**
 * This class defines a test for the VerifyConsensusNodeChangesetStep, which verifies that only valid repositories are
 * selected for deployment among those specified in the network operation.
 */
public class VerifyConsensusNodeChangesetStepTest {

    @Test
    @DisplayName("Check that only one repository is valid and selected for deployment")
    void testWithSingleValidSoftwareVersionSourceRepo() {
        NetworkOperation networkOperation =
                TemplateLoader.load(NetworkOperation.class, TemplateLoader.TEMPLATE_NETWORK_UPGRADE);
        NetworkOperationStatus status = executeStep(networkOperation);
        validateSuccessStatus(status, 1);
        validateSuccessConditions(networkOperation, status);
    }

    @Test
    @DisplayName("Check that multiple repositories are valid and any one (random) is selected for deployment")
    void testWithMultipleValidSoftwareVersionSourceRepo() {
        NetworkOperation networkOperation =
                TemplateLoader.load(NetworkOperation.class, TemplateLoader.TEMPLATE_NETWORK_UPGRADE);

        // repeat the valid entry since we don't have multiple valid repositories as of now
        ImageRepository validRepo = networkOperation
                .getSpec()
                .getNetworkUpgrade()
                .getChangesets()
                .getConsensusNodeChangeset()
                .getSoftwareVersionSource()
                .getImageRepositories()
                .reversed() // reverse to take the last one
                .getFirst();

        networkOperation
                .getSpec()
                .getNetworkUpgrade()
                .getChangesets()
                .getConsensusNodeChangeset()
                .getSoftwareVersionSource()
                .getImageRepositories()
                .add(validRepo);

        NetworkOperationStatus status = executeStep(networkOperation);
        validateSuccessStatus(status, 2);
        validateSuccessConditions(networkOperation, status);
    }

    @Test
    @DisplayName("Check that status message is added when no valid image is found")
    void testWithInvalidSoftwareVersionSourceRepo() {
        NetworkOperation networkOperation =
                TemplateLoader.load(NetworkOperation.class, TemplateLoader.TEMPLATE_NETWORK_UPGRADE);
        networkOperation
                .getSpec()
                .getNetworkUpgrade()
                .getChangesets()
                .getConsensusNodeChangeset()
                .getSoftwareVersionSource()
                .getImageRepositories()
                .removeLast(); // remove the valid entry at the end
        NetworkOperationStatus status = executeStep(networkOperation);

        // validate the status messages
        validateFailureConditions(networkOperation, status);
    }

    /**
     * Executes the VerifyConsensusNodeChangesetStep with a given network operation and returns its result status.
     *
     * @param networkOperation The NetworkOperation to be processed by the step.
     * @return The resulting NetworkOperationStatus after executing the step
     */
    private NetworkOperationStatus executeStep(NetworkOperation networkOperation) {
        ProcessorStep<NetworkOperation, NetworkOperationStatus> step = new VerifyConsensusNodeChangesetStep();
        NetworkOperationStatus status = new NetworkOperationStatusBuilder()
                .withPhase(NetworkOperationStatus.Phase.Pending)
                .build();

        @SuppressWarnings("unchecked")
        Context<NetworkOperation> context = Mockito.mock(Context.class);

        NetworkOperationStatus resp = step.run(networkOperation, status, context);

        assertThat(resp).isNotNull();
        assertThat(resp.getSoftwareSourceStatus()).isNotNull();
        assertThat(resp.getSoftwareSourceStatus().getConsensusNode().size())
                .isEqualTo(networkOperation
                        .getSpec()
                        .getNetworkUpgrade()
                        .getChangesets()
                        .getConsensusNodeChangeset()
                        .getSoftwareVersionSource()
                        .getImageRepositories()
                        .size());

        return resp;
    }

    /**
     * Validates that the status indicates success with a specific count of valid repositories selected for deployment.
     *
     * @param status The NetworkOperationStatus to validate.
     * @param expectedCount The expected number of valid repositories.
     */
    private void validateSuccessStatus(final NetworkOperationStatus status, int expectedCount) {
        List<ImageRepositoryStatus> valid = new ArrayList<>();
        ImageRepositoryStatus selected = null;
        for (ImageRepositoryStatus rs : status.getSoftwareSourceStatus().getConsensusNode()) {
            if (rs.getValidity().equals(ImageRepositoryStatus.Validity.Valid)) {
                valid.add(rs);

                if (rs.getSelectedForDeployment()) {
                    selected = rs;
                }
            }
        }

        assertThat(valid.size()).isEqualTo(expectedCount);
        assertThat(selected).isNotNull();
        assertThat(selected.getManifestHash()).isNotEmpty();
        assertThat(selected.getSelectedForDeployment()).isTrue();

        // order does not matter as any valid repository can be selected randomly
        assertThat(valid.contains(selected)).isTrue();
    }

    /**
     * Validates that the conditions in the status indicate success after running the step.
     *
     * @param networkOperation The original NetworkOperation used as input to the step.
     * @param status The resulting NetworkOperationStatus from executing the step.
     */
    private void validateSuccessConditions(NetworkOperation networkOperation, NetworkOperationStatus status) {
        assertThat(status.getConditions().size()).isEqualTo(1);
        assertThat(status.getConditions().getFirst().getMessage())
                .startsWith(VerifyConsensusNodeChangesetStep.SUCCESS_STATUS_MESSAGE.formatted(""));
    }

    /**
     * Validates that the status indicates failure due to no valid image repositories found.
     *
     * @param status The NetworkOperationStatus to validate.
     */
    private void validateFailStatus(final NetworkOperationStatus status) {
        List<ImageRepositoryStatus> valid = new ArrayList<>();
        ImageRepositoryStatus selected = null;
        for (ImageRepositoryStatus rs : status.getSoftwareSourceStatus().getConsensusNode()) {
            if (rs.getValidity().equals(ImageRepositoryStatus.Validity.Valid)) {
                valid.add(rs);

                if (rs.getSelectedForDeployment()) {
                    selected = rs;
                }
            }
        }

        assertThat(valid).isEqualTo(0);
        assertThat(selected).isNull();
    }

    /**
     * Validates that the conditions in the status indicate failure after running the step without valid repositories.
     *
     * @param networkOperation The original NetworkOperation used as input to the step.
     * @param status The resulting NetworkOperationStatus from executing the step.
     */
    private void validateFailureConditions(NetworkOperation networkOperation, NetworkOperationStatus status) {
        assertThat(status.getConditions().size()).isEqualTo(1);
        assertThat(status.getConditions().getFirst().getMessage()).isEqualTo(FAIL_STATUS_MESSAGE);
    }
}
