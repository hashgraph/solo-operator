import com.hedera.solo.operator.buildlogic.gitextractor.GitExtractor
import org.gradle.api.Project

fun Project.gitExtractor(initBlock: GitExtractor.() -> Unit = {}) {
    GitExtractor(this).apply(initBlock)
}
