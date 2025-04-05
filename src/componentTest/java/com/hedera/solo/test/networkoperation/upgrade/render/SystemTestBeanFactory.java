// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import com.fasterxml.jackson.core.type.TypeReference;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import com.hedera.solo.operator.test.fixtures.fuzz.KubernetesResourceFactory;
import java.lang.reflect.Type;
import java.nio.charset.StandardCharsets;
import java.util.List;

public class SystemTestBeanFactory extends KubernetesResourceFactory {
    public SystemTestBeanFactory() {
        this(null);
    }

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
