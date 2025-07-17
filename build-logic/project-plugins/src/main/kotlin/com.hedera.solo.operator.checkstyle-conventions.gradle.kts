// SPDX-License-Identifier: Apache-2.0

plugins { checkstyle }

checkstyle {
    toolVersion = "10.25.1"
    configFile = rootProject.file("config/checkstyle/checkstyle.xml")
    isShowViolations = true
}
