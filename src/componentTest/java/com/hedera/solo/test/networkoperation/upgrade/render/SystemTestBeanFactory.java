// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import com.fasterxml.jackson.core.type.TypeReference;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import com.hedera.solo.operator.test.fixtures.fuzz.KubernetesResourceFactory;
import java.lang.reflect.Type;
import java.nio.charset.StandardCharsets;
import java.util.List;

/**
 * The {@code SystemTestBeanFactory} class is a specific implementation of {@link KubernetesResourceFactory} designed
 * for system tests that require the generation of AddressBookEntry objects.
 */
public class SystemTestBeanFactory extends KubernetesResourceFactory {
    /** Constructs a new {@code SystemTestBeanFactory} with no seed provided. */
    public SystemTestBeanFactory() {
        this(null);
    }

    /**
     * Constructs a new {@code SystemTestBeanFactory} with the specified seed for random number generation.
     *
     * @param seed The seed to be used for generating random numbers.
     */
    public SystemTestBeanFactory(final Long seed) {
        super(seed);
        nullProbability(0.0);
        supplier(new TypeReference<List<AddressBookEntry>>() {}, this::addressBookEntries);
    }

    @Override
    protected String randomString(final Type type) {
        String randomString = super.randomString(type);
        if (randomString != null && !randomString.isEmpty()) {
            byte b = randomString.getBytes(StandardCharsets.UTF_8)[0];
            if ('a' <= b && b <= 'z') {
                return "_" + randomString;
            }
        }
        return randomString;
    }

    /**
     * Generates a list of {@link AddressBookEntry} objects.
     *
     * @param beanFactory The factory used to generate beans.
     * @param type The type reference for the list, specifically {@code List<AddressBookEntry>}.
     * @return A list of randomly generated or configured {@code AddressBookEntry} objects
     */
    List<AddressBookEntry> addressBookEntries(final BeanFactory beanFactory, final Type type) {
        List<AddressBookEntry> list = beanFactory
                .randoms(AddressBookEntry.class)
                .limit(random.nextInt(listBound))
                .toList();
        for (AddressBookEntry entry : list) {
            if (entry != null) {
                if (entry.getInternalPort() != null) {
                    entry.setInternalPort(entry.getInternalPort() & 0xffff);
                }
                if (entry.getExternalPort() != null) {
                    entry.setExternalPort(entry.getExternalPort() & 0xffff);
                }
            }
        }
        return list;
    }
}
