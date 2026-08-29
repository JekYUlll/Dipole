# Agent Release Manifest v1

`dipole.agent.release-manifest.v1` binds one Agent Runtime candidate to the model, Prompt, Capability Schema, Memory Policy and offline Eval Suite used to produce it.

The manifest is an evidence boundary, not a production switch. Candidates must pass `offline`, then `shadow`, then `user_gray`; the manifest stage is advanced by an operator after the corresponding evidence is reviewed. A promotion check requires the candidate and Eval Suite hash to match exactly and accepts only a `shadow` manifest.
