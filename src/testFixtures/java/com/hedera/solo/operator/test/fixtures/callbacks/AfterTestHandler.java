/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.test.fixtures.callbacks;

import static org.junit.platform.commons.support.AnnotationSupport.findAnnotatedMethods;
import static org.junit.platform.commons.support.HierarchyTraversalMode.BOTTOM_UP;

import com.hedera.solo.operator.test.fixtures.annotations.AfterTest;
import java.lang.reflect.InvocationTargetException;
import java.lang.reflect.Method;
import java.util.ArrayDeque;
import java.util.List;
import java.util.stream.Collector;
import java.util.stream.Stream;

public class AfterTestHandler {
    private AfterTestHandler() {}

    public static void afterTest(
            final List<Object> outer, final Object inner, final Method testMethod, final Throwable throwable) {
        ArrayDeque<?> instances = Stream.concat(outer.stream(), Stream.of(inner))
                .collect(Collector.of(ArrayDeque::new, ArrayDeque::addFirst, (a, b) -> {
                    b.addAll(a);
                    return b;
                }));
        for (Object instance : instances) {
            for (Method method : findAnnotatedMethods(instance.getClass(), AfterTest.class, BOTTOM_UP)) {
                try {
                    Class<?>[] types = method.getParameterTypes();
                    Object[] args = new Object[types.length];
                    for (int i = 0; i < types.length; i++) {
                        if (types[i].isAssignableFrom(Method.class)) {
                            args[i] = testMethod;
                        }
                        if (types[i].isAssignableFrom(Throwable.class)) {
                            args[i] = throwable;
                        }
                    }
                    method.setAccessible(true);
                    method.invoke(instance, args);
                } catch (IllegalAccessException e) {
                    throw new RuntimeException(e);
                } catch (InvocationTargetException e) {
                    throw new RuntimeException(e.getTargetException());
                }
            }
        }
    }
}
