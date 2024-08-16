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

package com.hedera.solo.operator.test.capsule;

import io.quarkus.test.junit.QuarkusTest;
import java.lang.invoke.MethodHandles;

@QuarkusTest
public class ConsensusCapsuleTest extends ConsensusCapsuleTestAbstract {
    public ConsensusCapsuleTest() {
        setNamespace(MethodHandles.lookup().lookupClass());
    }
}
