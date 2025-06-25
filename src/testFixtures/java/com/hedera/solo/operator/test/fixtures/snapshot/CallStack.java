// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

/** The CallStack class provides a utility method to retrieve the calling method's stack frame. */
public class CallStack {

    /**
     * Retrieves the stack frame of the method that called this method.
     *
     * @return StackWalker.StackFrame representing the calling method's stack frame
     */
    public static StackWalker.StackFrame calledFrom() {
        return StackWalker.getInstance(StackWalker.Option.RETAIN_CLASS_REFERENCE)
                .walk(s -> s.skip(2).findFirst())
                .orElseThrow();
    }
}
