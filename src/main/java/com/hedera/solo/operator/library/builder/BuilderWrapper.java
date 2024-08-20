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

package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.builder.Nested;

public class BuilderWrapper<T, B extends Builder<T>, E extends Editable<B>> {
    private final B builder;

    private BuilderWrapper(final B builder) {
        this.builder = builder;
    }

    public static <T, B extends Builder<T>, E extends Editable<B>> BuilderWrapper<T, B, E> edit(final E editable) {
        return new BuilderWrapper<>(editable.edit());
    }

    public BuilderWrapper<T, B, E> with(final BuilderFunction<B> function) {
        for (Object object = function.apply(builder);
                object instanceof Nested<?>;
                object = ((Nested<?>) object).and()) {}
        return this;
    }

    public T build() {
        return builder.build();
    }
}
