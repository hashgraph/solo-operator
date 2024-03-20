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

/**
 * A builder for the {@link OrbitSpec} class.
 *
 * @see OrbitSpec
 */
public class OrbitSpecBuilder extends OrbitSpecFluent<OrbitSpecBuilder>
        implements VisitableBuilder<OrbitSpec, OrbitSpecBuilder> {

    /** The fluent builder used to construct the {@code OrbitSpec}. */
    private final OrbitSpecFluent<?> fluent;

    /** Creates a new {@code OrbitSpecBuilder} with the default values. */
    public OrbitSpecBuilder() {
        this(new OrbitSpec());
    }

    /**
     * Creates a new {@code OrbitSpecBuilder} using the provided fluent builder.
     *
     * @param fluent the existing fluent builder to use when constructing the {@code OrbitSpec}.
     * @throws NullPointerException if the provided fluent builder is {@code null}.
     */
    public OrbitSpecBuilder(@NotNull final OrbitSpecFluent<?> fluent) {
        this(fluent, new OrbitSpec());
    }

    /**
     * Creates a new {@code OrbitSpecBuilder} from an existing instance.
     *
     * @param instance the existing instance used to populate the new builder.
     */
    public OrbitSpecBuilder(@Nullable final OrbitSpec instance) {
        this.fluent = this;
        this.copyInstance(instance);
    }

    /**
     * Creates a new {@code OrbitSpecBuilder} using the provided fluent builder and existing instance.
     *
     * @param fluent the existing fluent builder to use when constructing the {@code OrbitSpec}.
     * @param instance the existing instance used to populate the new builder.
     * @throws NullPointerException if the provided fluent builder is {@code null}.
     */
    public OrbitSpecBuilder(@NotNull final OrbitSpecFluent<?> fluent, @Nullable final OrbitSpec instance) {
        this.fluent = requireNonNull(fluent);
        this.copyInstance(instance);
    }

    /**
     * Builds the {@code OrbitSpec}. Guaranteed to return a non-{@code null} value.
     *
     * @return the constructed {@code OrbitSpec}.
     */
    @NotNull
    @Override
    public OrbitSpec build() {
        final OrbitSpec buildable = new OrbitSpec(fluent.getLifetime(), fluent.getFinalizers());
        buildable.setAdditionalProperties(fluent.getAdditionalProperties());
        return buildable;
    }
}
