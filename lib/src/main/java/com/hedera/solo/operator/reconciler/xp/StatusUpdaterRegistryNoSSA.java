package com.hedera.solo.operator.reconciler.xp;

import com.hedera.solo.operator.v1alpha1.StatusUpdater;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.OwnerReference;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.dsl.Gettable;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.api.reconciler.dependent.Deleter;
import io.javaoperatorsdk.operator.processing.dependent.AbstractDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.Matcher;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.GenericKubernetesResourceMatcher;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.ResourceUpdaterMatcher;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.updatermatcher.GenericResourceUpdaterMatcher;
import java.lang.invoke.MethodHandles;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE, resourceDiscriminator = StatusUpdaterRegistryNoSSA.class)
public class StatusUpdaterRegistryNoSSA extends CRUDKubernetesDependentResource<ConfigMap, StatusUpdater>
    implements ResourceUpdaterMatcher<ConfigMap>, ResourceDiscriminator<ConfigMap, StatusUpdater>
{
    public StatusUpdaterRegistryNoSSA() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(StatusUpdater primary, Context<StatusUpdater> context) {
        ConfigMap existing = context.getSecondaryResources(ConfigMap.class).stream().findFirst().orElse(null);
        if (null != existing) {
            existing = existing.edit().editMetadata().withManagedFields().endMetadata().build();
            return existing;
        }

        ConfigMap configMap = new ConfigMapBuilder()
                .editMetadata()
                .withNamespace(primary.getMetadata().getNamespace())
                .withName("status-updater-no-ssa")
                .endMetadata()
                .build();
        context.getClient().resources(StatusUpdater.class).inNamespace(primary.getMetadata().getNamespace()).resources()
                .map(Gettable::get)
                .forEach(configMap::addOwnerReference);
        return configMap;
    }

    @Override
    public Matcher.Result<ConfigMap> match(ConfigMap actualResource,
                                         StatusUpdater primary,
                                         Context<StatusUpdater> context) {
        return Result.computed(actualResource.hasOwnerReferenceFor(primary),
                actualResource.edit().editMetadata().withManagedFields().endMetadata().build());
    }

    ResourceUpdaterMatcher<ConfigMap> updater = GenericResourceUpdaterMatcher.updaterMatcherFor(ConfigMap.class);

    @Override
    public ConfigMap updateResource(ConfigMap actual, ConfigMap desired, Context<?> context) {
        ConfigMap updated = updater.updateResource(actual, desired, context);
        context.getPrimaryCache().list().forEach(updated::addOwnerReference);
        return updated;
    }

    @Override
    public boolean matches(ConfigMap actual, ConfigMap desired, Context<?> context) {
        return updater.matches(actual, desired, context) && context.getPrimaryCache().list().allMatch(actual::hasOwnerReferenceFor);
    }

    @Override
    public Optional<ConfigMap> distinguish(Class<ConfigMap> aClass, StatusUpdater statusUpdater, Context<StatusUpdater> context) {
        return context.getSecondaryResources(ConfigMap.class).stream().filter(x -> "status-updater-no-ssa".equals(x.getMetadata().getName())).findFirst();
    }
}
