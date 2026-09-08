# QUARTZ-17 Repair Receipt

This directory contains the low-sensitivity output from a Remote GPU isolated
Provider regression after the reply-memory usage rule was added.

- `receipt.json` records only runtime revision, SHA-256 identifiers, boolean
  canary results, model-call counts, and Memory lineage counts.
- `eval.json` evaluates the recall and revoke cases. The revoke reply text is
  observational because the short-term conversation may still include an older
  answer; the zero lineage count is the read-boundary authority.

The isolated Compose project used a unique Agent image tag and was removed at
completion. This is a single repaired synthetic sample, so it does not change
the default-disabled Memory policy or replace the full multi-canary window.
