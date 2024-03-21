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

package com.hedera.solo.operator.model.orbit;

import com.hedera.solo.operator.model.common.Condition;

public class OrbitCondition extends Condition<OrbitCondition, ?> {
    public OrbitCondition() {}

    public OrbitCondition(
            final String lastTransitionTime,
            final String message,
            final String reason,
            final String status,
            final String type) {
        super(lastTransitionTime, message, reason, status, type);
    }
}
