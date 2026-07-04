# ⚠️ TEST-ONLY KEYS — NOT SECRETS, NEVER FOR PRODUCTION

The `.pem` files in this directory are **well-known, publicly published test
keys** copied verbatim from the open-source
[hiero-local-node](https://github.com/hiero-ledger/hiero-local-node) project
(Apache-2.0). See `../../README.md` for provenance and the pinned commit.

They exist only so the locally-built consensus-node package is installable for
**UC sidecar testing**. They:

- are public and identical across every hiero-local-node checkout,
- protect nothing — no real network, account, or wallet uses them,
- **must NEVER be used in production or any non-local environment.**

A secret-scanner or security-audit hit on these files is a **false positive**;
this note is the justification for suppressing it.
