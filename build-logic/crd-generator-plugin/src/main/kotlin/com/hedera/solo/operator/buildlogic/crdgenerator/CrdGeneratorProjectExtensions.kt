// SPDX-License-Identifier: Apache-2.0

import com.hedera.solo.operator.buildlogic.crdgenerator.CrdGenerator
import org.gradle.api.Project

fun Project.crdGenerator(initBlock: CrdGenerator.() -> Unit = {}) {
    CrdGenerator(this).apply(initBlock)
}
