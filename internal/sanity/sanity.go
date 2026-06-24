// SPDX-License-Identifier: Apache-2.0

// Package sanity provides input validators for values that flow into filesystem
// paths or shell-adjacent contexts. It is a focused port of solo-weaver's
// pkg/sanity so the UC sidecar applies the same safety rules the consensus
// daemon does. Keep ValidateOperationID in sync with solo-weaver/pkg/sanity.
package sanity

import (
	"regexp"
	"strings"

	"github.com/joomcode/errorx"
)

// TimestampLayout is the compact ISO-8601 UTC layout embedded in the operationId
// convention "<type>-[<version>-]<timestamp>", e.g. "upgrade-20260415T143000Z",
// "migration-20260415T143000Z", or "upgrade-v0.76.0-20260415T143000Z". The
// <version> segment is optional; the <timestamp> is what the UC event-log
// retention orders and prunes by.
//
// This grammar is a CONVENTION, not a hard gate: ValidateOperationID enforces
// only path/shell safety, mirroring the consensus daemon. The format is produced
// by the orbit reconciler (the CR author); downstream consumers like the UC must
// not reject an id merely for deviating from the convention — they degrade
// (a timestampless id simply never gets pruned) rather than break the upgrade.
const TimestampLayout = "20060102T150405Z"

// shellMetachars contains dangerous shell metacharacters that must be rejected
// in any value that may be embedded in a filename or command.
var shellMetachars = regexp.MustCompile(`[;&|$\x60<>(){}[\]*?~]`)

// isValidIdentifierChar reports whether b is in the identifier set [A-Za-z0-9_-].
func isValidIdentifierChar(b byte) bool {
	return ('a' <= b && b <= 'z') ||
		('A' <= b && b <= 'Z') ||
		('0' <= b && b <= '9') ||
		b == '_' ||
		b == '-'
}

// isValidOperationIDChar reports whether b is valid in an operationId. The set is
// the identifier set plus '.', because operationIds may embed dotted version
// strings (e.g. "upgrade-v0.76.0-20060102T150405Z"). The '..' traversal sequence
// is rejected upstream in ValidateOperationID.
func isValidOperationIDChar(b byte) bool {
	return isValidIdentifierChar(b) || b == '.'
}

// ValidateOperationID validates a consensus-node operationId for path/shell
// safety. The id flows into archive and event-log filenames, so it must be safe
// to embed in a path: non-empty, [A-Za-z0-9_.-] only, no ".." traversal
// sequence, no shell metacharacters. The '.' is permitted so operationIds can
// embed dotted version strings (e.g. "upgrade-v0.76.0-20060102T150405Z"). It does
// NOT enforce the <type>-[<version>-]<timestamp> convention; see TimestampLayout.
func ValidateOperationID(s string) error {
	if s == "" {
		return errorx.IllegalArgument.New("operationId cannot be empty")
	}

	// Check for path traversal attempts before the per-char loop.
	if strings.Contains(s, "..") {
		return errorx.IllegalArgument.New("operationId contains path traversal sequences: %s", s)
	}

	// Check for shell metacharacters (narrower, more specific message).
	if shellMetachars.MatchString(s) {
		return errorx.IllegalArgument.New("operationId contains shell metacharacters: %s", s)
	}

	// Every byte must be a valid operationId character.
	for i := 0; i < len(s); i++ {
		if !isValidOperationIDChar(s[i]) {
			return errorx.IllegalArgument.New("operationId contains invalid characters: %s", s)
		}
	}

	return nil
}
