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

package com.hedera.solo.operator.orbit;

import static java.util.Objects.requireNonNull;

import io.fabric8.kubernetes.api.builder.VisitableBuilder;
import io.smallrye.common.constraint.NotNull;
import io.smallrye.common.constraint.Nullable;

public class OrbitSpecBuilder extends OrbitSpecFluent<OrbitSpecBuilder>
        implements VisitableBuilder<OrbitSpec, OrbitSpecBuilder> {

    private final OrbitSpecFluent<?> fluent;

    public OrbitSpecBuilder() {
        this(new OrbitSpec());
    }

    public OrbitSpecBuilder(final OrbitSpecFluent<?> fluent) {
        this(fluent, new OrbitSpec());
    }

    public OrbitSpecBuilder(@Nullable final OrbitSpec spec) {
        this.fluent = this;
        this.copyInstance(spec);
    }

    public OrbitSpecBuilder(@NotNull final OrbitSpecFluent<?> fluent, @Nullable final OrbitSpec instance) {
        this.fluent = requireNonNull(fluent);
        this.copyInstance(instance);
    }

    @Override
    public OrbitSpec build() {
        final OrbitSpec buildable = new OrbitSpec(fluent.getLifetime(), fluent.getFinalizers());
        buildable.setAdditionalProperties(fluent.getAdditionalProperties());
        return buildable;
    }
}
