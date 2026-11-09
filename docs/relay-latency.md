# Relay latency safeguards

## Early semantic SSE delivery

For an OpenAI API-key account, set `extra.openai_early_semantic_sse` to the boolean
`true` to flush complete semantic progress events, including reasoning-item
starts, without waiting for a later visible delta. It is off by default.

This changes delivery, not the semantic/visible TTFT definitions or model
completion time. Lifecycle-only events and incomplete JSON remain staged.
Once semantic progress is delivered, a later failure belongs to the same stream;
the gateway must not silently replay that attempt against another account.
Model, reasoning effort, prompt, tools and billing extraction are unchanged.

## Upstream admission pauses

A structured HTTP 429 with code `group_anti_distillation_paused` is returned
without pool-mode replay or account failover. Safe `Retry-After` headers survive;
ordinary capacity/authentication retry policies are unchanged. The pause is not
reported as an unhealthy model account.

Compare paired request clocks rather than two independent inference calls. A
relay's application ingress, final-forward start, semantic progress, visible
text and terminal event are different timing endpoints. Neither feature above
claims to eliminate an upstream's unreported admission/credential preparation.
