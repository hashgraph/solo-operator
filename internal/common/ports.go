// SPDX-License-Identifier: Apache-2.0

package common

// Hedera consensus node port numbers.
const (
	GossipPort = 50111
	GrpcPort   = 50211
	GrpcsPort  = 50212
)

// ConsensusSubdomain is the headless service name / StatefulSet subdomain for consensus nodes.
// Both the Orbit controller (headless service) and the ConsensusCapsule controller (StatefulSet)
// must use the same value for pod DNS to resolve correctly.
const ConsensusSubdomain = "consensus"
