// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-operator-solo-hedera-com-v1alpha1-networkupgradeprepare,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.solo.hedera.com,resources=networkupgradeprepares,verbs=update;delete,versions=v1alpha1,name=vnetworkupgradeprepare.kb.io,admissionReviewVersions=v1

type NetworkUpgradePrepareValidator struct{}

func (v *NetworkUpgradePrepareValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return builder.WebhookManagedBy[*NetworkUpgradePrepare](mgr, &NetworkUpgradePrepare{}).
		WithValidator(v).
		Complete()
}

func (v *NetworkUpgradePrepareValidator) ValidateCreate(_ context.Context, _ *NetworkUpgradePrepare) (admission.Warnings, error) {
	return nil, nil
}

func (v *NetworkUpgradePrepareValidator) ValidateUpdate(_ context.Context, old, _ *NetworkUpgradePrepare) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradePrepare %q is immutable after creation", old.Name)
}

func (v *NetworkUpgradePrepareValidator) ValidateDelete(_ context.Context, obj *NetworkUpgradePrepare) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradePrepare %q cannot be deleted", obj.Name)
}
