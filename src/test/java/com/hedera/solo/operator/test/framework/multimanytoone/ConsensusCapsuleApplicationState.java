package com.hedera.solo.operator.test.framework.multimanytoone;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class ConsensusCapsuleApplicationState extends AbstractApplicationState<ConsensusCapsule> {
//    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

//    @Override
//    protected InformerConfiguration.InformerConfigurationBuilder<ApplicationState> informerConfigurationBuilder() {
//        return super.informerConfigurationBuilder()
//                .withPrimaryToSecondaryMapper(p -> Set.of(new ResourceID("application-state", p.getMetadata().getNamespace())));
////                .withSecondaryToPrimaryMapper(
////                        r -> {
////                            log.error("withSecondaryToPrimaryMapper");
////                            return Collections.emptySet(); }
//////                                r.getMetadata().getOwnerReferences().stream()
//////                                .filter(x -> x.getKind().equals("ConsensusCapsule"))
//////                                .map(x -> new ResourceID(x.getName(), r.getMetadata().getNamespace())).collect(Collectors.toSet())
////                );
//    }

//    @Override
//    public Set<ResourceID> toPrimaryResourceIDs(ConsensusCapsule resource) {
//        return Set.of();
//    }
}
