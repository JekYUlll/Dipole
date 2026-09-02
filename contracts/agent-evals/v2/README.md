# Agent Evaluation Contract v2

`shadow-summary-report.schema.json` defines the public v2 aggregation envelope.
It omits Task, Run, and Trace identifiers; the evaluator retains Trace IDs only
inside protected summary input while validating uniqueness.

`shadow-manifest-set-receipt.schema.json` extends the v1 low-sensitivity
reviewed Shadow Eval window receipt with `minimumManifestCount`. The collector
records both the reviewer-required threshold and the observed manifest count,
so a later reader can distinguish a debugging run from a fixed-size window.

The v1 schema remains immutable for historical receipts. A v2 receipt does not
make a task-success claim by itself: reviewer labels, one candidate version,
unique terminal Task/Run reports and the complete window archive remain
required.
