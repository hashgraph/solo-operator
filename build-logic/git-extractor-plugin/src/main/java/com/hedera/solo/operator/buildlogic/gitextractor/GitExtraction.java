// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.Serializable;

/**
 * Represents a Git extraction with repository, refspec, source, and destination information.
 *
 * @param repository The remote git repository URL.
 * @param refspec The branch or tag reference to extract from the repository.
 * @param source The local directory where the extracted files will be placed.
 * @param destination The remote path within the repository where the extraction should occur.
 */
public record GitExtraction(String repository, String refspec, String source, String destination) implements Serializable {}
