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

import com.hedera.solo.operator.library.generic.Compare;
import io.quarkus.logging.Log;
import java.lang.invoke.MethodHandles;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class MyXTest {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    //    private final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    @Test
    public void test() {
        Log.debug("Hello World!");
        Log.info("Hello World!");
        Log.error("Hello World!");
        Compare.sub(3, 4);
    }
}
