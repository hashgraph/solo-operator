// SPDX-License-Identifier: Apache-2.0

package uc

import "github.com/joomcode/errorx"

// UC error taxonomy. Each error the UC sidecar returns is created from one of
// these types and decorated with a reason code + remediation hints via the
// internal/errx helpers (see that package for the convention). The errorx
// Temporary() trait marks errors the poll loop should retry; its absence marks a
// terminal condition that re-running cannot fix.
var (
	errNS = errorx.NewNamespace("uc")

	// errConfigInvalid: a UC flag/config value is missing or out of range.
	// Terminal — the process cannot start until the operator fixes the config.
	errConfigInvalid = errNS.NewType("config_invalid")

	// errManifestInvalid: the deployment-package manifest is unusable (unknown
	// schemaVersion, missing required field, malformed YAML). Terminal —
	// re-reading the same bytes will not help, so the handler halts without
	// creating a CR rather than retrying.
	errManifestInvalid = errNS.NewType("manifest_invalid")

	// errPersist: persisting or reading a CR under --cr-persist-path failed (a
	// disk/IO problem). Transient — the persist-before-send step is retried on the
	// next poll tick.
	errPersist = errNS.NewType("persist_failed", errorx.Temporary())

	// errK8sAPI: a Kubernetes API call (List/Create/Watch/Patch) failed.
	// Transient — the poll loop retries on the next tick.
	errK8sAPI = errNS.NewType("k8s_api_failed", errorx.Temporary())

	// errCRBuild: constructing or serialising a CR failed. This is a
	// programming/data error (e.g. an unconvertible object), not operator-fixable.
	errCRBuild = errNS.NewType("cr_build_failed")

	// errControl: a UC control-socket client call (status/health) failed.
	errControl = errNS.NewType("control_failed")

	// errClient: building the Kubernetes client / loading kubeconfig failed at
	// startup. Terminal until the cluster credentials/connectivity are fixed.
	errClient = errNS.NewType("k8s_client_failed")
)
