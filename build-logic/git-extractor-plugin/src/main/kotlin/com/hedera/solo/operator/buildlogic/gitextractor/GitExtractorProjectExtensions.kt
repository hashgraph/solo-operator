// SPDX-License-Identifier: Apache-2.0

import com.hedera.solo.operator.buildlogic.gitextractor.GitExtractor
import org.gradle.api.Project

fun Project.gitExtractor(initBlock: GitExtractor.() -> Unit = {}) {
    GitExtractor(this).apply(initBlock)
}
