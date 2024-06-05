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

//    static class StatusUpdaterReconcilerDiscriminator implements ResourceDiscriminator<Resources, StatusUpdater> {
//
//        @Override
//        public Optional<Resources> distinguish(Class<Resources> resource, StatusUpdater primary, Context<StatusUpdater> context) {
//            return context.eventSourceRetriever().getResourceEventSourceFor(St);
//        }
//    }
//        AbstractDependentResource<Resources, StatusUpdater> implements
//        Creator<Resources, StatusUpdater>,
//        Updater<Resources, StatusUpdater> {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    public StatusUpdaterRegistryNoSSA() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(StatusUpdater primary, Context<StatusUpdater> context) {
        ConfigMap existing = context.getSecondaryResources(ConfigMap.class).stream().findFirst().orElse(null);
        log.info("existing: {}", null != existing);
        if (null != existing) {
            log.error("owners: {}", existing.getMetadata().getOwnerReferences().stream().map(OwnerReference::getName).toList());
            existing = existing.edit().editMetadata().withManagedFields().endMetadata().build();
//            existing.addOwnerReference(primary);
            return existing;
        }

        // context.getSecondaryResources(Resources.class).iterator().next().hasOwnerReferenceFor(primary)
        // context.getSecondaryResources(Resources.class).iterator().next().addOwnerReference(primary)
//        List<String> names = context.getSecondaryResource(Resources.class)
//                .map(CustomResource::getSpec)
//                .map(ResourcesSpec::getNames)
//                .orElse(List.of());
//        String name = primary.getMetadata().getName();
//        if (!names.contains(name)) {
//            names = new ArrayList<>(names);
//            names.add(name);
//        }
        ConfigMap configMap = new ConfigMapBuilder()
                .editMetadata()
                .withNamespace(primary.getMetadata().getNamespace())
                .withName("status-updater-no-ssa")
                .endMetadata()
                .build();
        context.getClient().resources(StatusUpdater.class).inNamespace(primary.getMetadata().getNamespace()).resources()
                .map(Gettable::get)
//                .filter(Objects::nonNull) // <- ???
                .forEach(configMap::addOwnerReference);
        return configMap;
//                new ConfigMapBuilder()
//                .editMetadata()
//                .withNamespace(primary.getMetadata().getNamespace())
//                .withName("status-updater")
//                .endMetadata()
////                .editSpec()
////                .withNames(names)
////                .endSpec()
//                .build();
    }

//    @Override
//    public ConfigMap updateResource(ConfigMap actual, ConfigMap desired, Context<?> context) {
//        return null;
//    }
//
//    @Override
//    public boolean matches(ConfigMap actual, ConfigMap desired, Context<?> context) {
//        return false;
//    }

//    @Override
//    public Result<Resources> match(Resources resource, StatusUpdater primary, Context<StatusUpdater> context) {
//        log.info("StatusUpdaterRegistry::match");
//        return Result.nonComputed(false);
//    }
//
//    @Override
//    protected void onCreated(StatusUpdater primary, Resources created, Context<StatusUpdater> context) {
//        log.info("StatusUpdaterRegistry::onCreated");
//    }
//
//    @Override
//    protected void onUpdated(StatusUpdater primary, Resources updated, Resources actual, Context<StatusUpdater> context) {
//        log.info("StatusUpdaterRegistry::onUpdated");
//    }
//
//    @Override
//    protected Resources desired(StatusUpdater primary, Context<StatusUpdater> context) {
//        log.info("StatusUpdaterRegistry::desired");
//        return new ResourcesBuilder()
//                .editMetadata()
//                .withName("status-updater")
//                .withNamespace(primary.getMetadata().getNamespace())
//                .endMetadata()
//                .editSpec()
//                .withNames(primary.getMetadata().getName())
//                .endSpec()
//                .build();
//    }
//
//    @Override
//    public Class<Resources> resourceType() {
//        log.info("StatusUpdaterRegistry::resourceType");
//        return Resources.class;
//    }
//
//    @Override
//    public Resources update(Resources actual, Resources desired, StatusUpdater primary, Context<StatusUpdater> context) {
//        log.info("StatusUpdaterRegistry::update");
//        if (desired.getSpec().getNames().contains(primary.getMetadata().getName())) {
//            return desired;
//        }
//        return desired.edit().editSpec().addToNames(primary.getMetadata().getName()).endSpec().build();
//    }
//
//    @Override
//    public Resources create(Resources desired, StatusUpdater primary, Context<StatusUpdater> context) {
//        log.info("StatusUpdaterRegistry::create");
//        Resources updated = update(desired, desired, primary, context);
//        var rc = context.getClient().resource(updated);
//        return rc
//                .fieldManager(context.getControllerConfiguration().fieldManager())
//                .forceConflicts()
//                .serverSideApply();
////                .create();
////        return update(desired, desired, primary, context);
//    }

//    @Override
//    public Result<ConfigMap> match(ConfigMap actualResource, StatusUpdater primary, Context<StatusUpdater> context) {
//        return Result.nonComputed(false);
//    }
//    @Override
//    public Result<ConfigMap> match(ConfigMap actualResource, StatusUpdater primary, Context<StatusUpdater> context) {
//        var desired = dependentResource.desired(primary, context);
//        return match(desired, actualResource, false, false, false, context);
//    }
    @Override
    public Matcher.Result<ConfigMap> match(ConfigMap actualResource,
                                         StatusUpdater primary,
                                         Context<StatusUpdater> context) {
        log.error("matcher");
        return Result.computed(actualResource.hasOwnerReferenceFor(primary),
                actualResource.edit().editMetadata().withManagedFields().endMetadata().build());
    }

    ResourceUpdaterMatcher<ConfigMap> updater = GenericResourceUpdaterMatcher.updaterMatcherFor(ConfigMap.class);

    @Override
    public ConfigMap updateResource(ConfigMap actual, ConfigMap desired, Context<?> context) {
        log.info("updateResource");
        ConfigMap updated = updater.updateResource(actual, desired, context);
        context.getPrimaryCache().list().forEach(updated::addOwnerReference);
        return updated;
    }

    @Override
    public boolean matches(ConfigMap actual, ConfigMap desired, Context<?> context) {
        log.info("matches");
        return updater.matches(actual, desired, context) && context.getPrimaryCache().list().allMatch(actual::hasOwnerReferenceFor);
    }

    @Override
    public Optional<ConfigMap> distinguish(Class<ConfigMap> aClass, StatusUpdater statusUpdater, Context<StatusUpdater> context) {
        return context.getSecondaryResources(ConfigMap.class).stream().filter(x -> "status-updater-no-ssa".equals(x.getMetadata().getName())).findFirst();
    }
}
