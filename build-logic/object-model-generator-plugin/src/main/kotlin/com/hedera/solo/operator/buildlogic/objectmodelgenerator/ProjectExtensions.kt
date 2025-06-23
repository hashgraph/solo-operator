// SPDX-License-Identifier: Apache-2.0

import com.hedera.solo.operator.buildlogic.objectmodelgenerator.ObjectModelGenerator
import org.gradle.api.Project

public fun Project.objectModelGenerator(initBlock: ObjectModelGenerator.() -> Unit = {}) {
    ObjectModelGenerator(this).apply(initBlock)
}
