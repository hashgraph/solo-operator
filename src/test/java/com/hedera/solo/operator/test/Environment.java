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

package com.hedera.solo.operator.test;

import static java.net.http.HttpResponse.BodyHandlers.ofString;

import jakarta.enterprise.context.ApplicationScoped;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;

@ApplicationScoped
public class Environment {
    static final URI devModeEndpoint = URI.create("http://localhost:8080/q/arc");
    static final HttpClient httpClient = HttpClient.newHttpClient();
    static final HttpRequest devModeRequest =
            HttpRequest.newBuilder(devModeEndpoint).GET().build();

    public boolean isDevModeActive() {
        try {
            return httpClient.send(devModeRequest, ofString()).statusCode() == 200;
        } catch (IOException | InterruptedException e) {
            return false;
        }
    }
}
