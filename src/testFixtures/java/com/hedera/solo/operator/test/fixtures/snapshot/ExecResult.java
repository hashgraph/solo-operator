// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

/**
 * Represents the result of an execution command with additional details such as exit code, standard output (out), and
 * standard error (err).
 */
class ExecResult {
    public String[] exec;
    public int exitCode;
    public String out;
    public String err;

    /**
     * Constructs a new ExecResult object with the given parameters.
     *
     * @param exec The command and its arguments.
     * @param exitCode The exit code of the executed command.
     * @param out Standard output from the executed command.
     * @param err Standard error from the executed command.
     */
    ExecResult(final String[] exec, final int exitCode, final String out, final String err) {
        this.exec = exec;
        this.exitCode = exitCode;
        this.out = out;
        this.err = err;
    }
}
