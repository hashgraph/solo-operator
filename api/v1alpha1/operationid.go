// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// GetOperationID satisfies common.OperationIDer for reconcile logging.

func (o *NetworkUpgradePrepare) GetOperationID() string     { return o.Spec.OperationID }
func (o *NetworkUpgradeFreeze) GetOperationID() string      { return o.Spec.OperationID }
func (o *NetworkUpgradeExecute) GetOperationID() string     { return o.Spec.OperationID }
func (o *NetworkUpgradeFreezeAbort) GetOperationID() string { return o.Spec.OperationID }
func (o *NetworkGenesis) GetOperationID() string            { return o.Spec.OperationID }
