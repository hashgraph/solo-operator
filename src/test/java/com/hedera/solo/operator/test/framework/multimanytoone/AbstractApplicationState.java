package com.hedera.solo.operator.test.framework.multimanytoone;

import com.hedera.solo.operator.library.dependentresources.ManyToOneKubernetesDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ApplicationState;
import com.hedera.solo.operator.model.v1alpha1.ApplicationStateBuilder;
import io.fabric8.kubernetes.api.model.HasMetadata;

public abstract class AbstractApplicationState<P extends HasMetadata>
        extends ManyToOneKubernetesDependentResource<ApplicationState, P>
//        implements PrimaryToSecondaryMapper<P>
{
    public AbstractApplicationState() {
        super(ApplicationState.class);
    }

    @Override
    protected ApplicationState create(P primary) {
        return new ApplicationStateBuilder()
                .editOrNewMetadata()
                .withName("application-state")
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .build();
    }
//
//    @Override
//    public Set<ResourceID> toSecondaryResourceIDs(P primary) {
//        return Set.of(new ResourceID("application-state", primary.getMetadata().getNamespace()));
//    }
}
