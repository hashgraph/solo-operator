# Example manifests

## ⚠️ Example credentials — not secrets

`secrets.yaml` holds placeholder credentials so the examples run on a local
test network:

- S3 / RustFS access keys (`solo-operator` / `changeme`)
- the well-known `0.0.2` genesis operator key (`OPERATOR_KEY_MAIN`)

These values are public, shared by every local Solo network, and protect
nothing of value. Do **not** use them in production or to guard any real
account, bucket, or network. Replace them with your own values for any real
deployment.

Any secret-scanning alert on these files is a false positive.
