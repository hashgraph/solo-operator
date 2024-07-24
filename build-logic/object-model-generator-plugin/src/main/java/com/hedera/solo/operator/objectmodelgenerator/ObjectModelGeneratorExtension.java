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

package com.hedera.solo.operator.objectmodelgenerator;

import io.fabric8.java.generator.Config;

import java.io.File;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

public class ObjectModelGeneratorExtension {
    /**
     * Builder for the default configuration
     */
    public Config.ConfigBuilder defaultConfigBuilder = Config.builder();
    /**
     * Source directory for CRDs
     */
    public File sourceCrd;
    /**
     * Source directory for OpenAPI
     */
    public File sourceOpenAPI;
    /**
     * Target directory for generated CRDs
     */
    public File targetCrd;
    /**
     * Target directory for generated Java source
     */
    public File targetJava;
    /**
     * Builder override for specific file named by map key
     */
    public Map<String, Config.ConfigBuilder> configBuilderOverrides = new HashMap<>();
    /**
     * Optional set of generated files to remove.
     */
    public Set<File> removeFiles = new HashSet<>();
}
