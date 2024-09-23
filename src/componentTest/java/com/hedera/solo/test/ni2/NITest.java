package com.hedera.solo.test.ni2;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSequence;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import org.junit.jupiter.api.Test;

/*
*
could use:
primary=ni, secondary=next-ni
primary=next-ni secondary=ni

where next-ni names the next ni in its status, and ni only runs when it is
named by next-ni.

seems the initial implementation may be "simplest".
initial implementation requires some arbitary delays in the event there is a
backlog of nis.

actually, I can probably remove those arbitrary delays
* */

public class NITest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(KubernetesClient client) {
        return builder(client)
                .withReconciler(new NIRec())
                .withReconciler(new SeqRec());
    }

    @Test
    void test() {
        operator().create(edit(new NetworkInitializationSequence())
                .with(builder -> builder
                        .editMetadata()
                        .withName(operator().getNamespace())
                        .withNamespace(operator().getNamespace()))
                .build());

        for (int i = 0; i < 10; i++) {
            int count = i;
            operator().create(edit(new NetworkInitialization())
                    .with(builder -> builder
                            .editMetadata()
                            .withName("name-" + count))
                    .build());
        }

        try {
            Thread.sleep(9999);
        } catch (InterruptedException e) {
            throw new RuntimeException(e);
        }

        int i = 1;
    }
}
