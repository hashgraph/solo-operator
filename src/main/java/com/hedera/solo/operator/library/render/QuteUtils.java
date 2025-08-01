// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.render;

import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import io.quarkus.qute.Engine;
import io.quarkus.qute.ParserHelper;
import io.quarkus.qute.ParserHook;
import io.quarkus.qute.ReflectionValueResolver;
import io.quarkus.qute.Template;
import io.quarkus.qute.TemplateInstance;
import java.io.FileNotFoundException;
import java.io.IOException;
import java.io.InputStream;
import java.io.UncheckedIOException;
import java.nio.charset.Charset;
import java.nio.charset.StandardCharsets;
import java.util.function.Function;

/** Utility class for rendering Qute templates and generating Kubernetes resources. */
public class QuteUtils {

    /** Private constructor to prevent instantiation. */
    private QuteUtils() {}

    private static final Engine engine = Engine.builder()
            .addDefaults()
            .addValueResolver(new ReflectionValueResolver())
            .addParserHook(new ParserHook() {
                @Override
                public void beforeParsing(ParserHelper parserHelper) {
                    parserHelper.addContentFilter(contents -> contents.replace("${", "$\\{"));
                }
            })
            .build();
    private static final KubernetesSerialization kubernetesSerialization = new KubernetesSerialization();

    /**
     * Generates an instance of the specified type using the provided template.
     *
     * @param <T> the type of the object to generate
     * @param type the class type to generate
     * @param template the Qute Template instance
     * @return an instance of the specified type
     */
    public static <T> T generate(final Class<T> type, final Template template) {
        return generate(type, template, Function.identity());
    }

    /**
     * Generates an instance of the specified type using the provided template and configuration function.
     *
     * @param <T> the type of the object to generate
     * @param type the class type to generate
     * @param template the Qute Template instance
     * @param config a function to configure the TemplateInstance before rendering
     * @return an instance of the specified type
     */
    public static <T> T generate(
            final Class<T> type, final Template template, final Function<TemplateInstance, TemplateInstance> config) {
        return kubernetesSerialization.unmarshal(
                config.apply(template.instance()).render(), type);
    }

    /**
     * Loads a template from the classpath under "templates/" directory.
     *
     * @param loader the class from which to load the template
     * @param name the name of the template file
     * @return a Qute Template instance
     */
    public static Template template(final Class<?> loader, final String name) {
        return template(loader, name, StandardCharsets.UTF_8);
    }

    /**
     * Loads a template from the classpath under "templates/" directory.
     *
     * @param loader the class from which to load the template
     * @param name the name of the template file
     * @param charset the character set to use for reading the template file
     * @return a Qute Template instance
     */
    public static Template template(final Class<?> loader, final String name, final Charset charset) {
        try (InputStream is = loader.getResourceAsStream("/templates/" + name)) {
            if (is == null) {
                throw new FileNotFoundException("Template file not found: " + name);
            }
            return engine.parse(new String(is.readAllBytes(), charset));
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        }
    }
}
