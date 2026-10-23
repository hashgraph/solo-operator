// SPDX-License-Identifier: Apache-2.0

package v1alpha1

// FirewallPlan is the operator's declaration of the host-firewall intent for one
// upgrade: which peer gossip addresses a named allow rule must admit at each stage.
// It is intent, not a concrete firewall rule. The operator is backend-agnostic: it
// never produces nftables/iptables/cloud rules and never tracks what is applied.
// Translating the intent into real rules, resolving any hostnames, applying them
// idempotently and keeping the applied-rule state is the solo-provisioner CLI's job;
// the provisioner daemon only invokes the CLI and holds no roster logic of its own.
//
// The operator computes it in NetworkUpgradePrepare.status by diffing the candidate
// roster (spec.artifacts) against the last-applied roster, and copies it verbatim
// into NetworkUpgradeExecute.status when the Execute CR is created.
//
// Each list is the complete desired membership for a stage, never a diff, so the CLI
// converges on whichever list it is handed; a retry or a daemon restart reaches the
// same state without drift.
//
// Entries are "address:port", where address is an IPv4 or an FQDN taken from the
// roster's gossip endpoints and port is a single port or an inclusive range
// ("50111-50112"). A node's gossip endpoint is set by that node's own admin key, so
// an entry is attacker-influenced input to every peer's firewall: the operator must
// reject unspecified, loopback and multicast addresses before it writes the plan.
type FirewallPlan struct {
	// Rule is the logical allow rule the CLI maps to concrete host-firewall rules. The
	// CLI owns the rule's protocol and rejects names that collide with its own reserved
	// blocks; the operator only names the set.
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9_-]{0,31}$`
	Rule string `json:"rule"`

	// Rollback is the membership before this upgrade (the active roster's gossip
	// addresses). The CLI restores it if the upgrade is aborted.
	// +kubebuilder:validation:MaxItems=256
	// +kubebuilder:validation:items:MaxLength=266
	// +kubebuilder:validation:items:Pattern=`^[a-zA-Z0-9.-]+:[0-9]{1,5}(-[0-9]{1,5})?$`
	Rollback []string `json:"rollback"`

	// Transition is the membership to install before the node restarts: the union of
	// the active and candidate rosters, so a peer can reach a changed address while the
	// old one still works until the upgrade has succeeded.
	// +kubebuilder:validation:MaxItems=256
	// +kubebuilder:validation:items:MaxLength=266
	// +kubebuilder:validation:items:Pattern=`^[a-zA-Z0-9.-]+:[0-9]{1,5}(-[0-9]{1,5})?$`
	Transition []string `json:"transition"`

	// Target is the final membership, applied once the Execute CR has succeeded: the
	// candidate roster's gossip addresses only.
	// +kubebuilder:validation:MaxItems=256
	// +kubebuilder:validation:items:MaxLength=266
	// +kubebuilder:validation:items:Pattern=`^[a-zA-Z0-9.-]+:[0-9]{1,5}(-[0-9]{1,5})?$`
	Target []string `json:"target"`

	// RosterDigest is the sha256 of the candidate-roster artifact the plan was computed
	// from, tying the plan to exactly one roster.
	RosterDigest Sha256Digest `json:"rosterDigest"`

	// Digest is the sha256 over the canonical form of the fields above. It is a cheap
	// skip-hint so the daemon can avoid a no-op CLI call; it is not the correctness
	// mechanism. The authoritative applied state lives in the CLI, which converges on
	// the plan each time it runs.
	Digest Sha256Digest `json:"digest"`
}

// Condition types for the firewall change.
const (
	// CondFirewallPlanComputed is set on NetworkUpgradePrepare when the operator has
	// diffed the candidate roster and written status.firewallPlan (status True with a
	// reason of NoChange when the roster implies no firewall change).
	CondFirewallPlanComputed = "FirewallPlanComputed"

	// CondFirewallApplied is set on NetworkUpgradeExecute by the provisioner daemon
	// after it invokes the CLI. Its reason records the stage last applied: Transition,
	// Target or RolledBack. Status False is a terminal failure for the Execute CR, like
	// DaemonResult=False.
	CondFirewallApplied = "FirewallApplied"
)

// Reasons for CondFirewallApplied.
const (
	FirewallReasonTransition = "Transition"
	FirewallReasonTarget     = "Target"
	FirewallReasonRolledBack = "RolledBack"
	FirewallReasonNoChange   = "NoChange"
)
