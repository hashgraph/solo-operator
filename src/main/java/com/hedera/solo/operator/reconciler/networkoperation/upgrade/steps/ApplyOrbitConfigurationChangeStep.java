// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * ApplyOrbitConfigurationChangeStep is a step that applies the orbit level configuration changes
 *
 * <p>It rolls out config file bundle as specified in "configurationFiles" field and patch the orbit CR by making direct
 * API call to Kubernetes API server. The effect of the orbit CR changes will be the following that
 * {@link NetworkCheckProcessor}] will check:
 *
 * <ol>
 *   <li>Orbit reconciler creates config-maps: settings.txt, application.properties, api-permission.properties, etc.
 *   <li>Network-operation reconciler creates config.txt.
 * </ol>
 *
 * <p>If this step is re-invoked, the changes in orbit CR should be idempotent.
 */
public class ApplyOrbitConfigurationChangeStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public ApplyOrbitConfigurationChangeStep() {}

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        // TODO: Implement the logic to apply the orbit level configuration changes

        throw new ProcessorException("Not implemented");
    }
}
