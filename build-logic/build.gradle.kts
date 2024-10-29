// SPDX-License-Identifier: Apache-2.0

// pragma comment

tasks.register("check") {
    dependsOn(gradle.includedBuild("object-model-generator-plugin").task(":check"))
}

tasks.register("test") {
    dependsOn(gradle.includedBuild("object-model-generator-plugin").task(":test"))
}
