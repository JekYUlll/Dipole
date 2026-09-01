# Agent Evaluation Contract v2

`shadow-manifest-set-receipt.schema.json` extends the v1 low-sensitivity
reviewed Shadow Eval window receipt with `minimumManifestCount`. The collector
records both the reviewer-required threshold and the observed manifest count,
so a later reader can distinguish a debugging run from a fixed-size window.

The v1 schema remains immutable for historical receipts. A v2 receipt does not
make a task-success claim by itself: reviewer labels, one candidate version,
unique terminal Task/Run reports and the complete window archive remain
required.
