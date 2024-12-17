// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangesetBuilder;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSourceBuilder;
import com.hedera.solo.operator.model.commonspec.softwareversionsource.ImagesBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpecBuilder;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgradeBuilder;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.Changesets;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.ChangesetsBuilder;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyConsensusNodeChange;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyStreamUploaderChange;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.List;
import jdk.jfr.Description;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

class ExecuteImmediateProcessorTest {

    private NetworkOperation resource;
    private Context<NetworkOperation> context;
    private ExecuteImmediateProcessor processor;

    @SuppressWarnings("unchecked")
    @BeforeEach
    void setUp() {
        processor = new ExecuteImmediateProcessor();
        resource = Mockito.mock(NetworkOperation.class);
        context = (Context<NetworkOperation>) Mockito.mock(Context.class);
    }

    @Test
    @Description("Test prepareSteps method")
    void prepareSteps() {
        assertThrows(NullPointerException.class, () -> processor.prepareSteps(null, null));
        assertThrows(NullPointerException.class, () -> processor.prepareSteps(null, context));
        assertThrows(NullPointerException.class, () -> processor.prepareSteps(resource, null));

        List<ProcessorStep<NetworkOperation>> steps = processor.prepareSteps(resource, context);
        assertEquals(5, steps.size());
        assertInstanceOf(VerifyConsensusNodeChange.class, steps.getFirst());
        for (int i = 1; i < steps.size(); i++) {
            assertInstanceOf(VerifyStreamUploaderChange.class, steps.get(i));
        }
    }

    @Test
    @Description("Test process method consensus node changeset")
    void processConsensusNodeChangeset() {
        // prepare input resource
        final ConsensusNodeChangeset consensusNodeChangeset = new ConsensusNodeChangesetBuilder()
                .withSoftwareVersionSource(new SoftwareVersionSourceBuilder()
                        .withImages(new ImagesBuilder()
                                .withRepository("test.example.com")
                                .withName("test")
                                .withTag("0.0.1")
                                .build())
                        .build())
                .build();
        final Changesets changesets = new ChangesetsBuilder()
                .withConsensusNodeChangeset(consensusNodeChangeset)
                .build();
        final NetworkOperation resource = new NetworkOperationBuilder()
                .withSpec(new NetworkOperationSpecBuilder()
                        .withNetworkUpgrade(new NetworkUpgradeBuilder()
                                .withChangesets(changesets)
                                .build())
                        .build())
                .build();

        // process and get the desired resource with status
        final NetworkOperation desired = processor.process(resource, context);
        assertNotNull(desired);
        assertNotNull(desired.getStatus());
        assertNotNull(desired.getStatus().getConditions());
        assertEquals(
                ConditionStatus.True,
                desired.getStatus().getConditions().getFirst().getStatus(),
                "Condition status must be True");
        assertNotNull(desired.getStatus().getSoftwareSourceStatus(), "SoftwareSourceStatus must not be null");
    }

    @Test
    void addSuccessCondition() {}

    @Test
    void addFailureCondition() {}
}
