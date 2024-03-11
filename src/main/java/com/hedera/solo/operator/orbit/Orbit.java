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

import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.model.annotation.Group;
import io.fabric8.kubernetes.model.annotation.Version;

/**
 * An {@code Orbit} is a Kubernetes Custom Resource Definition (CRD) which defines a single self-contained Hedera
 * network running on one or more independent Kubernetes clusters.
 *
 * <p>Orbits are managed by the {@link OrbitReconciler}, which is responsible for the following tasks:
 *
 * <ul>
 *   <li>Managing a unique namespace based on the {@code Orbit} definition.
 *   <li>Managing configuration maps for all network and cluster wide settings:
 *       <ul>
 *         <li>Address Book
 *         <li>Node Configuration
 *         <li>Log4j Configuration
 *         <li>Telemetry Configuration
 *       </ul>
 *   <li>Managing version information for deployed resources.
 *   <li>Verifying the integrity of the software artifacts.
 * </ul>
 */
@Version("v1alpha1")
@Group("solo.hedera.com")
public class Orbit extends CustomResource<OrbitSpec, OrbitStatus> {

    /** Creates a new {@code Orbit} with default {@link OrbitSpec} and {@link OrbitStatus} instances. */
    public Orbit() {
        setSpec(new OrbitSpec());
        setStatus(new OrbitStatus());
    }
}
