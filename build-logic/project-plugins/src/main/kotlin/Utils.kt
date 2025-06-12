// SPDX-License-Identifier: Apache-2.0

import org.gradle.api.file.Directory
import org.gradle.api.file.RegularFile
import java.io.File
import java.io.OutputStream
import java.io.PrintStream

class Utils {
    companion object {
        @JvmStatic
        fun Directory.versionFile(): RegularFile =
            file("version.txt").let { if (it.asFile.exists()) it else this.dir("..").versionFile() }

        @JvmStatic
        fun generateProjectVersionReport(version: String, ostream: OutputStream) {
            val writer = PrintStream(ostream, false, Charsets.UTF_8)

            ostream.use {
                writer.use {
                    // Writer headers
                    writer.println("### Deployed Version Information")
                    writer.println()
                    writer.println("| Artifact Name | Version Number |")
                    writer.println("| --- | --- |")
                    // Write table rows
                    writer.printf("| %s | %s |\n", "hedera-node", version)
                    writer.printf("| %s | %s |\n", "platform-sdk", version)
                    writer.flush()
                    ostream.flush()
                }
            }
        }

        @JvmStatic
        fun updateStringInFile(file: File, startsWith: String, newString: String, ignoreLeadingSpace: Boolean = true) {
            var lines: List<String> = mutableListOf()

            if (file.exists()) {
                lines = file.readLines(Charsets.UTF_8)
            }

            val finalLines: List<String> = if (lines.isNotEmpty()) {
                lines.map {
                    if (ignoreLeadingSpace && it.trimStart().startsWith(startsWith)) {
                        newString
                    } else if (it.startsWith(startsWith)) {
                        newString
                    } else {
                        it
                    }
                }
            } else {
                listOf(newString)
            }

            file.bufferedWriter(Charsets.UTF_8).use { writer ->
                finalLines.forEach { lines ->
                    writer.write(lines)
                    writer.newLine()
                }
                writer.flush()
            }
        }
    }
}
