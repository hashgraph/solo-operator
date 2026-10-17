// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// FirewallProto is the L4 protocol a firewall allow rule matches. It mirrors the
// protocols solo-provisioner's host firewall accepts (`network firewall create-allow-rule --proto`).
// +kubebuilder:validation:Enum=tcp;udp
type FirewallProto string

const (
	FirewallProtoTCP FirewallProto = "tcp"
	FirewallProtoUDP FirewallProto = "udp"
)

// FirewallPlan is the operator-computed host firewall change for one upgrade: the
// membership the named allow rule must hold at each stage of the upgrade, so the
// provisioner daemon applies it with `network firewall set` and holds no roster
// logic of its own.
//
// The operator computes it in NetworkUpgradePrepare.status by diffing the
// candidate roster (spec.artifacts) against the last-applied roster, and copies it
// verbatim into NetworkUpgradeExecute.status when the Execute CR is created. The
// daemon only reads it.
//
// Each list is the complete desired membership of the rule, never a diff. `set`
// replaces the whole list atomically and is idempotent, so a retry or a daemon
// restart converges on the same state without drift.
//
// Entries are IP addresses, CIDRs or FQDNs, as the host firewall accepts. The
// operator must reject unspecified, loopback and multicast addresses before it
// writes the plan: the daemon applies whatever the plan names. A node's gossip
// endpoint is set by that node's own admin key, so this list is attacker-influenced
// input to every peer's firewall and is limited to the gossip port(s) below.
type FirewallPlan struct {
	// Rule is the named allow rule on the host firewall. It must not be one of the
	// reserved blocks (mgmt, blocked, in_cluster).
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9_-]{0,31}$`
	// +kubebuilder:validation:XValidation:rule="!(self in ['mgmt','blocked','in_cluster'])",message="rule must not be a reserved firewall block"
	Rule string `json:"rule"`

	// Proto is the protocol the rule's ports match. Defaults to tcp.
	// +optional
	// +kubebuilder:default=tcp
	Proto FirewallProto `json:"proto,omitempty"`

	// Ports are the destination ports (or inclusive ranges like "50111-50112")
	// the rule admits. Taken from the roster's gossip endpoints.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:items:Pattern=`^[0-9]{1,5}(-[0-9]{1,5})?$`
	Ports []string `json:"ports"`

	// Rollback is the membership before this upgrade (the active roster's gossip
	// addresses). The daemon restores it if the upgrade is aborted.
	// +kubebuilder:validation:MaxItems=256
	// +kubebuilder:validation:items:MaxLength=253
	Rollback []string `json:"rollback"`

	// Transition is the membership to install before the node restarts: the union
	// of the active and candidate rosters, so a peer can reach a changed address
	// and the old one still works until the upgrade has succeeded.
	// +kubebuilder:validation:MaxItems=256
	// +kubebuilder:validation:items:MaxLength=253
	Transition []string `json:"transition"`

	// Target is the final membership, applied once the Execute CR has succeeded:
	// the candidate roster's gossip addresses only.
	// +kubebuilder:validation:MaxItems=256
	// +kubebuilder:validation:items:MaxLength=253
	Target []string `json:"target"`

	// RosterDigest is the sha256 of the candidate-roster artifact the plan was
	// computed from, tying the plan to exactly one roster.
	RosterDigest Sha256Digest `json:"rosterDigest"`

	// Digest is the sha256 over the canonical form of the fields above. The daemon
	// records the last digest it applied and skips the change when it matches, so
	// a re-delivered plan is a no-op.
	Digest Sha256Digest `json:"digest"`
}

// Condition types for the firewall change.
const (
	// CondFirewallPlanComputed is set on NetworkUpgradePrepare when the operator has
	// diffed the candidate roster and written status.firewallPlan (status True with a
	// reason of NoChange when the roster implies no firewall change).
	CondFirewallPlanComputed = "FirewallPlanComputed"

	// CondFirewallApplied is set on NetworkUpgradeExecute by the provisioner daemon.
	// Its reason records the stage last applied: Transition, Target or RolledBack.
	// Status False is a terminal failure for the Execute CR, like DaemonResult=False.
	CondFirewallApplied = "FirewallApplied"
)

// Reasons for CondFirewallApplied.
const (
	FirewallReasonTransition = "Transition"
	FirewallReasonTarget     = "Target"
	FirewallReasonRolledBack = "RolledBack"
	FirewallReasonNoChange   = "NoChange"
)
