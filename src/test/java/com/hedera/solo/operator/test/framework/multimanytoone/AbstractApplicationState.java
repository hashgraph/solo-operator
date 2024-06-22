package com.hedera.solo.operator.test.framework.multimanytoone;

import com.hedera.solo.operator.library.dependentresources.ManyToOneKubernetesDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ApplicationState;
import com.hedera.solo.operator.model.v1alpha1.ApplicationStateBuilder;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.processing.event.ResourceID;

public abstract class AbstractApplicationState<P extends HasMetadata>
        extends ManyToOneKubernetesDependentResource<ApplicationState, P>
{
    public AbstractApplicationState() {
        super(ApplicationState.class);
    }

    @Override
    protected ApplicationState createSecondaryResource(P primary) {
        return new ApplicationStateBuilder()
                .editOrNewMetadata()
                .withName("application-state")
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .build();
    }

    @Override
    protected ResourceID secondaryResourceID(P primary) {
        return new ResourceID("application-state", primary.getMetadata().getNamespace());
    }
}
