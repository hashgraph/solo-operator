package com.hedera.solo.operator.test.fixtures;

import static java.net.http.HttpResponse.BodyHandlers.ofString;

import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;

public class Environment {
    static final URI devModeEndpoint = URI.create("http://localhost:8080/q/arc");
    static final HttpClient httpClient = HttpClient.newHttpClient();
    static final HttpRequest devModeRequest = HttpRequest.newBuilder(devModeEndpoint).GET().build();
    public static boolean isDevModeActive() {
        try {
            return httpClient.send(devModeRequest, ofString()).statusCode() == 200;
        } catch (IOException | InterruptedException e) {
            return false;
        }
    }
}
