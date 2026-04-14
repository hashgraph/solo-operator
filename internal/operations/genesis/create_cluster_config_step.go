// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/common"
)

// CreateClusterConfigStep creates the cluster configuration ConfigMap
type CreateClusterConfigStep struct {
	client client.Client
}

// NewCreateClusterConfigStep creates a new CreateClusterConfigStep
func NewCreateClusterConfigStep(c client.Client) *CreateClusterConfigStep {
	return &CreateClusterConfigStep{client: c}
}

// Name returns the step name
func (s *CreateClusterConfigStep) Name() string {
	return "CreateClusterConfig"
}

// Run creates/updates the cluster configuration ConfigMap
func (s *CreateClusterConfigStep) Run(ctx context.Context, resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) *GenesisStatus {
	logger := log.FromContext(ctx)

	namespace := resource.Namespace
	orbitName := resource.Spec.Orbit

	// Get the orbit
	orbit := &operatorv1alpha1.Orbit{}
	if err := s.client.Get(ctx, client.ObjectKey{Name: orbitName}, orbit); err != nil {
		logger.Error(err, "Failed to get orbit", "orbit", orbitName)
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Orbit not found: %s", orbitName))
	}

	// List consensus capsules
	capsuleList := &operatorv1alpha1.ConsensusCapsuleList{}
	if err := s.client.List(ctx, capsuleList, client.InNamespace(namespace)); err != nil {
		logger.Error(err, "Failed to list consensus capsules")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to list consensus capsules: %v", err))
	}

	if len(capsuleList.Items) == 0 {
		return s.markFailed(status, resource.Generation, fmt.Sprintf("No consensus capsules found in namespace: %s", namespace))
	}

	// List pods to get IPs (optional - genesis can run without pods)
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/managed-by": "solo-operator",
			"app.kubernetes.io/part-of":    "consensus-node",
			"app.kubernetes.io/component":  "deployment",
		},
	}
	if err := s.client.List(ctx, podList, listOpts...); err != nil {
		logger.Error(err, "Failed to list pods")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to list pods: %v", err))
	}

	// Collect ready pod IPs (optional - genesis can proceed without them)
	podIPs := s.collectPodIPs(podList)
	logger.Info("Pod status for genesis",
		"expected", len(capsuleList.Items),
		"found", len(podIPs))
	// Genesis can proceed without pods - config will be created for capsules to use

	// Build config.txt content
	configTxt := s.buildConfigTxt(capsuleList.Items, orbit, namespace)

	// Build data map with all required config files
	data := map[string]string{
		ConfigKeyConfigTxt:          configTxt,
		ConfigKeyPodIPs:             strings.Join(podIPs, "\n"),
		"settings.txt":              s.buildSettingsTxt(orbit),
		"application.properties":    s.buildApplicationProperties(orbit),
		"api-permission.properties": s.buildApiPermissionProperties(orbit),
		"bootstrap.properties":      s.buildBootstrapProperties(orbit),
		"genesis-throttles.json":    s.buildGenesisThrottles(orbit),
	}

	// Create or update the ConfigMap
	if err := s.createOrUpdateConfigMap(ctx, namespace, orbitName, data); err != nil {
		logger.Error(err, "Failed to create/update cluster config ConfigMap")
		return s.markFailed(status, resource.Generation, fmt.Sprintf("Failed to create cluster config ConfigMap: %v", err))
	}

	logger.Info("Successfully created/updated cluster config ConfigMap", "namespace", namespace)
	return s.markSucceeded(status, resource.Generation, namespace)
}

// collectPodIPs collects IPs from ready pods
func (s *CreateClusterConfigStep) collectPodIPs(podList *corev1.PodList) []string {
	var ips []string
	for _, pod := range podList.Items {
		// Skip pods being deleted
		if pod.DeletionTimestamp != nil {
			continue
		}

		// Check if pod is running
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Check if pod is ready but not fully ready (transitioning state we need)
		// The Java implementation checks for Ready=False while Running
		isReady := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionFalse {
				isReady = true
				break
			}
		}

		if !isReady {
			continue
		}

		if pod.Status.PodIP != "" {
			ips = append(ips, pod.Status.PodIP)
		}
	}
	return ips
}

// buildConfigTxt builds the config.txt content for the Hedera network
func (s *CreateClusterConfigStep) buildConfigTxt(capsules []operatorv1alpha1.ConsensusCapsule, orbit *operatorv1alpha1.Orbit, namespace string) string {
	var sb strings.Builder

	// Get shardId/realmId from orbit (defaults to 0/0)
	shardId := 0
	realmId := 0

	// Get ledger ID from orbit - use plain format for config.txt (not hex)
	ledgerId := "0" // default
	if orbit.Spec.Consensus.Genesis != nil && orbit.Spec.Consensus.Genesis.AddressBook != nil {
		ab := orbit.Spec.Consensus.Genesis.AddressBook
		shardId = ab.ShardId
		realmId = ab.RealmId
		if ab.LedgerId != "" {
			ledgerId = ab.LedgerId
			if strings.HasPrefix(ledgerId, "0x") {
				if ledgerId == "0x01" {
					ledgerId = "298"
				} else {
					ledgerId = strings.TrimPrefix(ledgerId, "0x")
				}
			}
		}
	}

	fmt.Fprintf(&sb, "swirld, %s\n", ledgerId)
	sb.WriteString("app, HederaNode.jar\n")

	// Build address entries from capsules (sorted by node ID)
	addressMap := make(map[int64]string)

	for _, capsule := range capsules {
		nodeId := capsule.Spec.NodeId
		nodeName := capsule.Name

		externalAddr := s.computeNodeExternalAddress(&capsule, nodeName, namespace)

		weight := int64(capsule.Spec.Weight)
		if weight == 0 {
			weight = 1
		}

		accountId := capsule.Spec.AccountId
		if accountId == "" {
			accountId = fmt.Sprintf("%d.%d.%d", shardId, realmId, 3+nodeId)
		}

		gossipPort := capsule.Spec.GossipPort
		if gossipPort == 0 {
			gossipPort = common.GossipPort
		}

		// Format: address, nodeId, nickname, selfname, weight, internalIP, internalPort, externalHost, externalPort, accountId
		addressMap[nodeId] = fmt.Sprintf("address, %d, %s, %s, %d, 127.0.0.1, %d, %s, %d, %s\n",
			nodeId, nodeName, nodeName, weight, gossipPort, externalAddr, gossipPort, accountId)
	}

	// Add external addresses from orbit if present
	if orbit.Spec.Consensus.Genesis != nil && orbit.Spec.Consensus.Genesis.AddressBook != nil {
		for _, entry := range orbit.Spec.Consensus.Genesis.AddressBook.ExternalAddresses {
			nodeId := entry.NodeId
			weight := entry.Weight
			if weight == 0 {
				weight = 1
			}
			addressMap[nodeId] = fmt.Sprintf("address, %d, %d, %s, %d, %s, %d, %s, %d, %s\n",
				nodeId,
				nodeId,
				entry.NickName,
				weight,
				entry.InternalHostname,
				entry.InternalPort,
				entry.ExternalHostname,
				entry.ExternalPort,
				entry.NodeAccount)
		}
	}

	// Sort by node ID and append
	var nodeIds []int64
	for id := range addressMap {
		nodeIds = append(nodeIds, id)
	}
	sort.Slice(nodeIds, func(i, j int) bool { return nodeIds[i] < nodeIds[j] })

	for _, id := range nodeIds {
		sb.WriteString(addressMap[id])
	}

	return sb.String()
}

// computeNodeExternalAddress computes the external address for a consensus node
func (s *CreateClusterConfigStep) computeNodeExternalAddress(capsule *operatorv1alpha1.ConsensusCapsule, nodeName, namespace string) string {
	if capsule.Spec.ExternalAddress != "" {
		return capsule.Spec.ExternalAddress
	}
	// Default: node-svc.namespace
	return fmt.Sprintf("%s-svc.%s", nodeName, namespace)
}

// buildSettingsTxt builds the settings.txt content.
// Defaults are merged with orbit.Spec.Consensus.NetworkSettings — orbit values override defaults.
func (s *CreateClusterConfigStep) buildSettingsTxt(orbit *operatorv1alpha1.Orbit) string {
	settings := map[string]string{
		"checkSignedStateFromDisk":              "1",
		"csvFileName":                           "MainNetStats",
		"deployment":                            "hashsphere",
		"doUpnp":                                "false",
		"event.creation.maxCreationRate":        "20",
		"loadKeysFromPfxFiles":                  "1",
		"maxEventQueueForCons":                  "1000",
		"maxOutgoingSyncs":                      "1",
		"merkleDb.hashesRamToDiskThreshold":     "8388608",
		"merkleDb.maxFileChannelsPerFileReader": "1",
		"reconnect.active":                      "1",
		"reconnect.reconnectWindowSeconds":      "-1",
		"showInternalStats":                     "1",
		"state.mainClassNameOverride":           "com.hedera.services.ServicesMain",
		"state.saveStatePeriod":                 "900",
		"useLoopbackIp":                         "false",
		"virtualMap.familyThrottleThreshold":    "6000000000",
		"waitAtStartup":                         "false",
	}
	for k, v := range orbit.Spec.Consensus.NetworkSettings {
		settings[k] = v
	}
	return renderSettingsTxt(settings)
}

// ledgerIdToHex converts a plain ledger ID string to the hex format used by
// application.properties / bootstrap.properties, and returns the matching chainId.
func ledgerIdToHex(raw string) (ledgerId, chainId string) {
	if strings.HasPrefix(raw, "0x") {
		switch raw {
		case "0x01":
			return raw, "298"
		case "0x00":
			return raw, "0"
		default:
			return raw, raw[2:]
		}
	}
	switch raw {
	case "298":
		return "0x01", "298"
	case "0":
		return "0x00", "0"
	default:
		return "0x" + raw, raw
	}
}

// buildApplicationProperties builds the application.properties content.
// Defaults are merged with orbit.Spec.Consensus.ApplicationConfiguration — orbit values override defaults.
func (s *CreateClusterConfigStep) buildApplicationProperties(orbit *operatorv1alpha1.Orbit) string {
	ledgerId := "0x00"
	chainId := "0"
	if orbit.Spec.Consensus.Genesis != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook.LedgerId != "" {
		ledgerId, chainId = ledgerIdToHex(orbit.Spec.Consensus.Genesis.AddressBook.LedgerId)
	}

	props := map[string]string{
		"hedera.config.version":                       "0",
		"ledger.id":                                   ledgerId,
		"netty.mode":                                  "TEST",
		"contracts.chainId":                           chainId,
		"hedera.recordStream.logPeriod":               "1",
		"balances.exportPeriodSecs":                   "400",
		"files.maxSizeKb":                             "2048",
		"hedera.recordStream.compressFilesOnCreation": "true",
		"balances.compressOnCreation":                 "true",
		"contracts.maxNumWithHapiSigsAccess":          "0",
		"autoRenew.targetTypes":                       "",
		"nodes.gossipFqdnRestricted":                  "false",
		"hedera.profiles.active":                      "TEST",
		"staking.periodMins":                          "1",
		"nodes.updateAccountIdAllowed":                "true",
		"blockStream.streamMode":                      "RECORDS",
		"scheduling.longTermEnabled":                  "false",
		"bootstrap.throttleDefsJson.resource":         "data/config/genesis-throttles.json",
		"cache.warmThreads":                           "6",
	}
	for k, v := range orbit.Spec.Consensus.ApplicationConfiguration {
		props[k] = v
	}
	return renderPropertiesSorted(props)
}

// buildApiPermissionProperties builds the api-permission.properties content.
// Defaults are merged with orbit.Spec.Consensus.PermissionConfiguration — orbit values override defaults.
func (s *CreateClusterConfigStep) buildApiPermissionProperties(orbit *operatorv1alpha1.Orbit) string {
	perms := map[string]string{
		"appendContent":              "0-*",
		"approveAllowances":          "0-*",
		"contractCallLocalMethod":    "0-*",
		"contractCallMethod":         "0-*",
		"contractGetBytecode":        "0-*",
		"createAccount":              "0-*",
		"createContract":             "0-*",
		"createFile":                 "0-*",
		"createTopic":                "0-*",
		"cryptoDelete":               "0-*",
		"cryptoGetBalance":           "0-*",
		"cryptoTransfer":             "0-*",
		"deleteAllowances":           "0-*",
		"deleteContract":             "0-*",
		"deleteFile":                 "0-*",
		"deleteTopic":                "0-*",
		"ethereumTransaction":        "0-*",
		"freeze":                     "2-58",
		"getAccountDetails":          "2-50",
		"getAccountInfo":             "0-*",
		"getAccountRecords":          "0-*",
		"getContractInfo":            "0-*",
		"getFileContent":             "0-*",
		"getFileInfo":                "0-*",
		"getTopicInfo":               "0-*",
		"getTransactionReceipts":     "0-*",
		"getTxRecordByContractID":    "0-*",
		"getTxRecordByTxID":          "0-*",
		"getVersionInfo":             "0-*",
		"networkGetExecutionTime":    "2-50",
		"scheduleCreate":             "0-*",
		"scheduleDelete":             "0-*",
		"scheduleGetInfo":            "0-*",
		"scheduleSign":               "0-*",
		"submitMessage":              "0-*",
		"systemDelete":               "2-59",
		"systemUndelete":             "2-60",
		"tokenAccountWipe":           "0-*",
		"tokenAssociateToAccount":    "0-*",
		"tokenBurn":                  "0-*",
		"tokenCreate":                "0-*",
		"tokenDelete":                "0-*",
		"tokenDissociateFromAccount": "0-*",
		"tokenFeeScheduleUpdate":     "0-*",
		"tokenFreezeAccount":         "0-*",
		"tokenGetAccountNftInfos":    "0-*",
		"tokenGetInfo":               "0-*",
		"tokenGetNftInfo":            "0-*",
		"tokenGetNftInfos":           "0-*",
		"tokenGrantKycToAccount":     "0-*",
		"tokenMint":                  "0-*",
		"tokenPause":                 "0-*",
		"tokenRevokeKycFromAccount":  "0-*",
		"tokenUnfreezeAccount":       "0-*",
		"tokenUnpause":               "0-*",
		"tokenUpdate":                "0-*",
		"updateAccount":              "0-*",
		"updateContract":             "0-*",
		"updateFile":                 "0-*",
		"updateTopic":                "0-*",
		"utilPrng":                   "0-*",
	}
	for k, v := range orbit.Spec.Consensus.PermissionConfiguration {
		perms[k] = v
	}
	return renderPropertiesSorted(perms)
}

// buildBootstrapProperties builds the bootstrap.properties content.
// Defaults are merged with orbit.Spec.Consensus.ApplicationConfiguration — orbit values override defaults.
// bootstrap.properties shares the same override map as application.properties so that a single
// ApplicationConfiguration entry (e.g. netty.mode) applies to both files consistently.
func (s *CreateClusterConfigStep) buildBootstrapProperties(orbit *operatorv1alpha1.Orbit) string {
	ledgerId := "0x00"
	chainId := "298"
	if orbit.Spec.Consensus.Genesis != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook != nil &&
		orbit.Spec.Consensus.Genesis.AddressBook.LedgerId != "" {
		ledgerId, chainId = ledgerIdToHex(orbit.Spec.Consensus.Genesis.AddressBook.LedgerId)
	}

	props := map[string]string{
		"ledger.id":                                   ledgerId,
		"netty.mode":                                  "DEV",
		"contracts.chainId":                           chainId,
		"hedera.recordStream.logPeriod":               "1",
		"balances.exportPeriodSecs":                   "400",
		"files.maxSizeKb":                             "2048",
		"hedera.recordStream.compressFilesOnCreation": "true",
		"balances.compressOnCreation":                 "true",
		"contracts.maxNumWithHapiSigsAccess":          "0",
	}
	for k, v := range orbit.Spec.Consensus.ApplicationConfiguration {
		props[k] = v
	}
	return renderPropertiesSorted(props)
}

// defaultThrottlesJSON is the built-in genesis-throttles.json used when no orbit override is provided.
const defaultThrottlesJSON = `{
  "buckets": [
    {
      "burstPeriod": 0,
      "burstPeriodMs": 15000,
      "name": "ThroughputLimits",
      "throttleGroups": [
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": [
            "ScheduleCreate",
            "CryptoCreate",
            "CryptoTransfer",
            "CryptoUpdate",
            "CryptoDelete",
            "CryptoGetInfo",
            "CryptoGetAccountRecords",
            "ConsensusCreateTopic",
            "ConsensusSubmitMessage",
            "ConsensusUpdateTopic",
            "ConsensusDeleteTopic",
            "ConsensusGetTopicInfo",
            "TokenGetNftInfo",
            "TokenGetInfo",
            "ScheduleDelete",
            "ScheduleGetInfo",
            "FileGetContents",
            "FileGetInfo",
            "ContractUpdate",
            "ContractDelete",
            "ContractGetInfo",
            "ContractGetBytecode",
            "ContractGetRecords",
            "ContractCallLocal",
            "TransactionGetRecord",
            "GetVersionInfo",
            "TokenGetAccountNftInfos",
            "TokenGetNftInfos",
            "CryptoApproveAllowance",
            "CryptoDeleteAllowance",
            "UtilPrng",
            "NodeCreate",
            "NodeUpdate",
            "NodeDelete"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 40000,
          "operations": [
            "FileCreate",
            "FileUpdate",
            "FileAppend",
            "FileDelete"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": [
            "ScheduleSign"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": [
            "TokenMint"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 35000,
          "operations": [
            "ContractCall",
            "ContractCreate",
            "EthereumTransaction"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": [
            "TokenCreate",
            "TokenDelete",
            "TokenBurn",
            "TokenUpdate",
            "TokenFeeScheduleUpdate",
            "TokenAssociateToAccount",
            "TokenAccountWipe",
            "TokenDissociateFromAccount",
            "TokenFreezeAccount",
            "TokenUnfreezeAccount",
            "TokenGrantKycToAccount",
            "TokenRevokeKycFromAccount",
            "TokenPause",
            "TokenUnpause",
            "TokenUpdateNfts",
            "TokenReject",
            "TokenAirdrop",
            "TokenClaimAirdrop",
            "TokenCancelAirdrop"
          ]
        }
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 1000,
      "name": "OffHeapQueryLimits",
      "throttleGroups": [
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": [
            "FileGetContents",
            "FileGetInfo",
            "ContractGetInfo",
            "ContractGetBytecode",
            "ContractCallLocal"
          ]
        }
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 3000,
      "name": "PriorityReservations",
      "throttleGroups": [
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 40000,
          "operations": [
            "FileCreate",
            "FileUpdate",
            "FileAppend",
            "FileDelete"
          ]
        }
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 15000,
      "name": "CreationLimits",
      "throttleGroups": [
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 40000,
          "operations": [
            "CryptoCreate",
            "NodeCreate"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 5000,
          "operations": [
            "ConsensusCreateTopic"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": [
            "TokenCreate",
            "TokenAssociateToAccount",
            "ScheduleCreate",
            "TokenAirdrop"
          ]
        }
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 1000,
      "name": "FreeQueryLimits",
      "throttleGroups": [
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 1000000000,
          "operations": [
            "TransactionGetReceipt"
          ]
        }
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 1000,
      "name": "BalanceQueryLimits",
      "throttleGroups": [
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 1000000,
          "operations": [
            "CryptoGetAccountBalance"
          ]
        }
      ]
    }
  ]
}
`

// buildGenesisThrottles returns the genesis-throttles.json content.
// If orbit.Spec.Consensus.Genesis.ThrottlesJson is set it is used verbatim;
// otherwise the built-in defaults are returned.
func (s *CreateClusterConfigStep) buildGenesisThrottles(orbit *operatorv1alpha1.Orbit) string {
	if orbit.Spec.Consensus.Genesis != nil && orbit.Spec.Consensus.Genesis.ThrottlesJson != "" {
		return orbit.Spec.Consensus.Genesis.ThrottlesJson
	}
	return defaultThrottlesJSON
}

// renderPropertiesSorted renders a key=value properties map in sorted key order.
func renderPropertiesSorted(props map[string]string) string {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%s=%s\n", k, props[k])
	}
	return sb.String()
}

// renderSettingsTxt renders a settings map as space-aligned settings.txt lines in sorted key order.
func renderSettingsTxt(settings map[string]string) string {
	keys := make([]string, 0, len(settings))
	for k := range settings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%-46s %s\n", k+",", settings[k])
	}
	return sb.String()
}

// createOrUpdateConfigMap creates or updates the cluster config ConfigMap
func (s *CreateClusterConfigStep) createOrUpdateConfigMap(ctx context.Context, namespace, orbitName string, data map[string]string) error {
	logger := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := s.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ClusterConfigName}, cm)

	if apierrors.IsNotFound(err) {
		// Create new ConfigMap
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ClusterConfigName,
				Namespace: namespace,
				Labels:    common.StandardLabels(ClusterConfigName, orbitName),
			},
			Data: data,
		}
		logger.Info("Creating cluster config ConfigMap", "namespace", namespace, "name", ClusterConfigName)
		return s.client.Create(ctx, cm)
	} else if err != nil {
		return err
	}

	// Update existing ConfigMap
	cm.Data = data
	cm.Labels = common.StandardLabels(ClusterConfigName, orbitName)
	logger.Info("Updating cluster config ConfigMap", "namespace", namespace, "name", ClusterConfigName)
	return s.client.Update(ctx, cm)
}

// markSucceeded marks the step as successful
func (s *CreateClusterConfigStep) markSucceeded(status *GenesisStatus, generation int64, namespace string) *GenesisStatus {
	now := metav1.Now()
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionTrue,
		LastTransitionTime: now,
		Reason:             StepConditionReasonClusterConfig,
		Message:            fmt.Sprintf("Successfully created cluster config: %s/%s", namespace, ClusterConfigName),
		ObservedGeneration: generation,
	})
	// Mark genesis as succeeded since this is the final step
	status.Phase = operatorv1alpha1.PhaseSucceeded
	return status
}

// markFailed marks the step as failed
func (s *CreateClusterConfigStep) markFailed(status *GenesisStatus, generation int64, message string) *GenesisStatus {
	now := metav1.Now()
	status.Phase = operatorv1alpha1.PhaseFailed
	status.AddCondition(metav1.Condition{
		Type:               StepConditionTypeConfigCreated,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: now,
		Reason:             StepConditionReasonClusterConfig,
		Message:            message,
		ObservedGeneration: generation,
	})
	return status
}

// IsSuccessful checks if this step has completed successfully
func (s *CreateClusterConfigStep) IsSuccessful(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	return status.HasSuccessfulCondition(StepConditionTypeConfigCreated, StepConditionReasonClusterConfig)
}

// ShouldSkip determines if this step should be skipped
func (s *CreateClusterConfigStep) ShouldSkip(resource *operatorv1alpha1.NetworkOperation, status *GenesisStatus) bool {
	if status.Phase == operatorv1alpha1.PhaseFailed {
		return true
	}
	return s.IsSuccessful(resource, status)
}
