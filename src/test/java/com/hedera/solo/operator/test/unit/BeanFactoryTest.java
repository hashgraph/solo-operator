// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.unit;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.core.type.TypeReference;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import java.util.Objects;
import java.util.Set;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;

public class BeanFactoryTest {
    BeanFactory factory = new BeanFactory();

    @Test
    void test() {
        Set<String> charset = Stream.of(0x274c, 0x2B55).map(Character::toString).collect(Collectors.toSet());
        assertThat(factory.charset(charset)
                        .nonNull(new TypeReference<Stream<String>>() {})
                        .filter(Objects::nonNull)
                        .limit(10)
                        .flatMap(s -> s.codePoints().mapToObj(Character::toString))
                        .distinct())
                .allMatch(charset::contains);
    }
}
