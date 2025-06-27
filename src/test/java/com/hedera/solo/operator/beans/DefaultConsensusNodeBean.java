// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.beans;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.ConsensusNode;
import jakarta.enterprise.context.Dependent;
import org.eclipse.microprofile.config.inject.ConfigProperty;

/**
 * The DefaultConsensusNodeBean class is a CDI-aware bean that provides configuration properties for the consensus node.
 *
 * <p>It uses MicroProfile Config to inject configuration values related to the consensus node's image repository, name,
 * and tag.
 */
@Dependent
public class DefaultConsensusNodeBean {
    @ConfigProperty(name = "solo.operator.consensus-node.image.repository")
    String repository;

    @ConfigProperty(name = "solo.operator.consensus-node.image.image-name")
    String imageName;

    @ConfigProperty(name = "solo.operator.consensus-node.image.image-tag")
    String imageTag;

    /**
     * Constructs a default ConsensusNode object using the injected configuration properties.
     *
     * @return A new instance of ConsensusNode configured with repository, imageName, and imageTag values
     */
    public ConsensusNode defaultConsensusNode() {
        return edit(new ConsensusNode())
                .with(image -> image.editSoftwareVersion()
                        .withRepository(repository)
                        .withImageName(imageName)
                        .withImageTag(imageTag))
                .build();
    }
}
