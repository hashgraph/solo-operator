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

package com.hedera.solo.operator;

import io.javaoperatorsdk.operator.api.config.LeaderElectionConfiguration;
import io.quarkus.arc.Unremovable;
import jakarta.enterprise.context.ApplicationScoped;

@ApplicationScoped
@Unremovable
public class LeaderElection extends LeaderElectionConfiguration {

    public static final String LEASE_NAME = "solo-operator-lease";
    public static final String LEASE_NAMESPACE = "default";

    public LeaderElection() {
        super(LEASE_NAME, LEASE_NAMESPACE);
    }
}
