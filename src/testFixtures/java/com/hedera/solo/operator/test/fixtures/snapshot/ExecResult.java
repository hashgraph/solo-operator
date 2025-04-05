// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

class ExecResult {
    public String[] exec;
    public int exitCode;
    public String out;
    public String err;

    public ExecResult(final String[] exec, final int exitCode, final String out, final String err) {
        this.exec = exec;
        this.exitCode = exitCode;
        this.out = out;
        this.err = err;
    }
}
