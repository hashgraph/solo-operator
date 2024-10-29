// SPDX-License-Identifier: Apache-2.0

plugins { id("com.diffplug.spotless") }

spotless {
    java {
        targetExclude("build/generated/**/*.java")
        // enable toggle comment support
        toggleOffOn()
        // don't need to set target, it is inferred from java
        // apply a specific flavor of google-java-format
        palantirJavaFormat("2.40.0").formatJavadoc(true)
        // make sure every file has the following copyright header.
        // optionally, Spotless can set copyright years by digging
        // through git history (see "license" section below).
        // The delimiter override below is required to support some
        // of our test classes which are in the default package.
        licenseHeader("// SPDX-License-Identifier: Apache-2.0\n\n", "(package|import)")
    }
}
