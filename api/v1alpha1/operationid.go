// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// GetOperationID exposes spec.operationId on the CRs that carry one, satisfying
// common.OperationIDer so reconcile logging can stamp operation_id automatically.

func (o *NetworkUpgradePrepare) GetOperationID() string     { return o.Spec.OperationID }
func (o *NetworkUpgradeFreeze) GetOperationID() string      { return o.Spec.OperationID }
func (o *NetworkUpgradeExecute) GetOperationID() string     { return o.Spec.OperationID }
func (o *NetworkUpgradeFreezeAbort) GetOperationID() string { return o.Spec.OperationID }
func (o *NetworkGenesis) GetOperationID() string            { return o.Spec.OperationID }
