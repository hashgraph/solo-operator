package com.hedera.solo.operator.reconciler.xp;

import io.fabric8.kubernetes.api.model.Namespaced;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.model.annotation.Group;
import io.fabric8.kubernetes.model.annotation.Kind;
import io.fabric8.kubernetes.model.annotation.Version;

@Group("lib.operator.solo.hedera.com")
@Version("v1")
@Kind("StatusUpdater")
public class StatusUpdater extends CustomResource<StatusUpdater.StatusUpdaterSpec, StatusUpdater.StatusUpdaterStatus>
        implements Namespaced {
    public record StatusUpdaterSpec() {}
    public record StatusUpdaterStatus(long count) {}
}
