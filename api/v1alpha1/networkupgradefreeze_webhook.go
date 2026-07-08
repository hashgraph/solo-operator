// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-operator-solo-hedera-com-v1alpha1-networkupgradefreeze,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.solo.hedera.com,resources=networkupgradefreezes,verbs=update;delete,versions=v1alpha1,name=vnetworkupgradefreeze.kb.io,admissionReviewVersions=v1

type NetworkUpgradeFreezeValidator struct{}

func (v *NetworkUpgradeFreezeValidator) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return builder.WebhookManagedBy[*NetworkUpgradeFreeze](mgr, &NetworkUpgradeFreeze{}).
		WithValidator(v).
		Complete()
}

func (v *NetworkUpgradeFreezeValidator) ValidateCreate(_ context.Context, _ *NetworkUpgradeFreeze) (admission.Warnings, error) {
	return nil, nil
}

func (v *NetworkUpgradeFreezeValidator) ValidateUpdate(_ context.Context, old, _ *NetworkUpgradeFreeze) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradeFreeze %q is immutable after creation", old.Name)
}

func (v *NetworkUpgradeFreezeValidator) ValidateDelete(_ context.Context, obj *NetworkUpgradeFreeze) (admission.Warnings, error) {
	return nil, fmt.Errorf("NetworkUpgradeFreeze %q cannot be deleted", obj.Name)
}
