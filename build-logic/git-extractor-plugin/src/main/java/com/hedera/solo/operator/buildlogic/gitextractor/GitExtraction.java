package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.File;

public record GitExtraction(
    String repository,
    String reference,
    String source,
    File destination){
}
