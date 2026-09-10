// SPDX-License-Identifier: Apache-2.0

package genesis

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1alpha1 "github.com/hashgraph/solo-operator/api/v1alpha1"
	"github.com/hashgraph/solo-operator/internal/resources/consensus"
)

// BuildClusterConfig builds the legacy cluster-config ConfigMap. config.txt
// is superseded by genesis-network.json but emitted for unmigrated consumers.
func BuildClusterConfig(orbit *operatorv1alpha1.Orbit, materials []RosterMaterial, podIPs []string) map[string]string {
	return map[string]string{
		ConfigKeyConfigTxt:          buildConfigTxt(orbit, materials),
		ConfigKeyPodIPs:             strings.Join(podIPs, "\n"),
		"settings.txt":              buildSettingsTxt(orbit),
		"application.properties":    buildApplicationProperties(orbit),
		"api-permission.properties": buildApiPermissionProperties(orbit),
		"bootstrap.properties":      buildBootstrapProperties(orbit),
		"genesis-throttles.json":    buildGenesisThrottles(orbit),
	}
}

func CollectPodIPs(ctx context.Context, c client.Client, namespace string) ([]string, error) {
	pods := &corev1.PodList{}
	if err := c.List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	ips := make([]string, 0)
	for _, pod := range pods.Items {
		if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" {
			continue
		}
		ready := false
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}
		if ready {
			ips = append(ips, pod.Status.PodIP)
		}
	}
	// c.List does not guarantee an order; sort so the derived pod-ips ConfigMap content
	// is deterministic for the same set of pods (avoids spurious rewrites).
	sort.Strings(ips)
	return ips, nil
}

// buildConfigTxt assumes materials are sorted by NodeID.
func buildConfigTxt(orbit *operatorv1alpha1.Orbit, materials []RosterMaterial) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "swirld, %s\n", orbitChainID(orbit))
	sb.WriteString("app, HederaNode.jar\n")

	for _, m := range materials {
		external := m.ExternalAddress
		if external == "" {
			external = m.GossipHost
		}
		fmt.Fprintf(&sb,
			"address, %d, %s, %s, %d, %s, %d, %s, %d, %s\n",
			m.NodeID,
			m.Description, m.Description,
			m.Weight,
			m.GossipHost, m.GossipPort,
			external, m.GossipPort,
			m.AccountID,
		)
	}
	return sb.String()
}

// orbitLedgerID returns the network's ledger identity (application.properties
// ledger.id), defaulting to mainnet's 0x00 when unset.
func orbitLedgerID(orbit *operatorv1alpha1.Orbit) string {
	if v := orbit.Spec.Consensus.Genesis.AddressBook.LedgerId; v != "" {
		return v
	}
	return "0x00"
}

// orbitChainID returns the network's EVM chain id (contracts.chainId / relay
// CHAIN_ID / the config.txt swirld id). It is a DECIMAL number (295/296/297/298),
// not a hex byte like ledger.id — so the unset default is decimal "0", not "0x00".
func orbitChainID(orbit *operatorv1alpha1.Orbit) string {
	if v := orbit.Spec.Consensus.Genesis.AddressBook.ChainId; v != "" {
		return v
	}
	return "0"
}

func buildSettingsTxt(_ *operatorv1alpha1.Orbit) string {
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
	return consensus.RenderSettingsTxt(settings)
}

func buildApplicationProperties(orbit *operatorv1alpha1.Orbit) string {
	overrides := map[string]string{"ledger.id": orbitLedgerID(orbit), "contracts.chainId": orbitChainID(orbit)}
	return consensus.MergeApplicationProperties(overrides)
}

func buildApiPermissionProperties(orbit *operatorv1alpha1.Orbit) string {
	perms := map[string]string{
		"appendContent": "0-*", "approveAllowances": "0-*", "contractCallLocalMethod": "0-*", "contractCallMethod": "0-*",
		"contractGetBytecode": "0-*", "createAccount": "0-*", "createContract": "0-*", "createFile": "0-*",
		"createTopic": "0-*", "cryptoDelete": "0-*", "cryptoGetBalance": "0-*", "cryptoTransfer": "0-*",
		"deleteAllowances": "0-*", "deleteContract": "0-*", "deleteFile": "0-*", "deleteTopic": "0-*",
		"ethereumTransaction": "0-*", "freeze": "2-58", "getAccountDetails": "2-50", "getAccountInfo": "0-*",
		"getAccountRecords": "0-*", "getContractInfo": "0-*", "getFileContent": "0-*", "getFileInfo": "0-*",
		"getTopicInfo": "0-*", "getTransactionReceipts": "0-*", "getTxRecordByContractID": "0-*", "getTxRecordByTxID": "0-*",
		"getVersionInfo": "0-*", "networkGetExecutionTime": "2-50", "scheduleCreate": "0-*", "scheduleDelete": "0-*",
		"scheduleGetInfo": "0-*", "scheduleSign": "0-*", "submitMessage": "0-*", "systemDelete": "2-59",
		"systemUndelete": "2-60", "tokenAccountWipe": "0-*", "tokenAssociateToAccount": "0-*", "tokenBurn": "0-*",
		"tokenCreate": "0-*", "tokenDelete": "0-*", "tokenDissociateFromAccount": "0-*", "tokenFeeScheduleUpdate": "0-*",
		"tokenFreezeAccount": "0-*", "tokenGetAccountNftInfos": "0-*", "tokenGetInfo": "0-*", "tokenGetNftInfo": "0-*",
		"tokenGetNftInfos": "0-*", "tokenGrantKycToAccount": "0-*", "tokenMint": "0-*", "tokenPause": "0-*",
		"tokenRevokeKycFromAccount": "0-*", "tokenUnfreezeAccount": "0-*", "tokenUnpause": "0-*", "tokenUpdate": "0-*",
		"updateAccount": "0-*", "updateContract": "0-*", "updateFile": "0-*", "updateTopic": "0-*", "utilPrng": "0-*",
	}
	return consensus.RenderProperties(perms)
}

func buildBootstrapProperties(orbit *operatorv1alpha1.Orbit) string {
	props := map[string]string{
		"ledger.id":                                   orbitLedgerID(orbit),
		"netty.mode":                                  "DEV",
		"contracts.chainId":                           orbitChainID(orbit),
		"hedera.recordStream.logPeriod":               "1",
		"balances.exportPeriodSecs":                   "400",
		"files.maxSizeKb":                             "2048",
		"hedera.recordStream.compressFilesOnCreation": "true",
		"balances.compressOnCreation":                 "true",
		"contracts.maxNumWithHapiSigsAccess":          "0",
	}
	return consensus.RenderProperties(props)
}

func buildGenesisThrottles(_ *operatorv1alpha1.Orbit) string {
	return defaultThrottlesJSON
}

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
            "ScheduleCreate", "CryptoCreate", "CryptoTransfer", "CryptoUpdate", "CryptoDelete",
            "CryptoGetInfo", "CryptoGetAccountRecords", "ConsensusCreateTopic", "ConsensusSubmitMessage",
            "ConsensusUpdateTopic", "ConsensusDeleteTopic", "ConsensusGetTopicInfo", "TokenGetNftInfo",
            "TokenGetInfo", "ScheduleDelete", "ScheduleGetInfo", "FileGetContents", "FileGetInfo",
            "ContractUpdate", "ContractDelete", "ContractGetInfo", "ContractGetBytecode", "ContractGetRecords",
            "ContractCallLocal", "TransactionGetRecord", "GetVersionInfo", "TokenGetAccountNftInfos",
            "TokenGetNftInfos", "CryptoApproveAllowance", "CryptoDeleteAllowance", "UtilPrng",
            "NodeCreate", "NodeUpdate", "NodeDelete"
          ]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 40000,
          "operations": ["FileCreate", "FileUpdate", "FileAppend", "FileDelete"]
        },
        {"opsPerSec": 0, "milliOpsPerSec": 100000, "operations": ["ScheduleSign"]},
        {"opsPerSec": 0, "milliOpsPerSec": 100000, "operations": ["TokenMint"]},
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 35000,
          "operations": ["ContractCall", "ContractCreate", "EthereumTransaction"]
        },
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": [
            "TokenCreate", "TokenDelete", "TokenBurn", "TokenUpdate", "TokenFeeScheduleUpdate",
            "TokenAssociateToAccount", "TokenAccountWipe", "TokenDissociateFromAccount",
            "TokenFreezeAccount", "TokenUnfreezeAccount", "TokenGrantKycToAccount",
            "TokenRevokeKycFromAccount", "TokenPause", "TokenUnpause", "TokenUpdateNfts",
            "TokenReject", "TokenAirdrop", "TokenClaimAirdrop", "TokenCancelAirdrop"
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
          "operations": ["FileGetContents", "FileGetInfo", "ContractGetInfo", "ContractGetBytecode", "ContractCallLocal"]
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
          "operations": ["FileCreate", "FileUpdate", "FileAppend", "FileDelete"]
        }
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 15000,
      "name": "CreationLimits",
      "throttleGroups": [
        {"opsPerSec": 0, "milliOpsPerSec": 40000, "operations": ["CryptoCreate", "NodeCreate"]},
        {"opsPerSec": 0, "milliOpsPerSec": 5000, "operations": ["ConsensusCreateTopic"]},
        {
          "opsPerSec": 0,
          "milliOpsPerSec": 100000,
          "operations": ["TokenCreate", "TokenAssociateToAccount", "ScheduleCreate", "TokenAirdrop"]
        }
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 1000,
      "name": "FreeQueryLimits",
      "throttleGroups": [
        {"opsPerSec": 0, "milliOpsPerSec": 1000000000, "operations": ["TransactionGetReceipt"]}
      ]
    },
    {
      "burstPeriod": 0,
      "burstPeriodMs": 1000,
      "name": "BalanceQueryLimits",
      "throttleGroups": [
        {"opsPerSec": 0, "milliOpsPerSec": 1000000, "operations": ["CryptoGetAccountBalance"]}
      ]
    }
  ]
}
`
