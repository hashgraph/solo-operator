# Nightly test fixtures

## ⚠️ Test-only credentials — not secrets

`secrets.yaml` holds placeholder credentials for the nightly test network:

- S3 / RustFS access keys (`solo-operator` / `changeme`)
- the well-known `0.0.2` genesis operator key (`OPERATOR_KEY_MAIN`)

These values are public, shared by every local Solo network, and protect
nothing of value. Do **not** use them in production or to guard any real
account, bucket, or network.

Any secret-scanning alert on these files is a false positive.
