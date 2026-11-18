# DeepSeek Harness / Antigravity Responses compatibility

The per-account `extra.antigravity_deepseek_harness_system_as_context` boolean is
**off by default**. Enable it only with the administrator's informed approval.

For Antigravity OAuth accounts serving `/v1/responses`, the option changes the
role of a system/developer input message whose first text starts exactly with
`You are an AI agent powered by DeepSeek Harness.` to `user`. Its complete
content, position and additional fields are preserved. Other system/developer
instructions, user messages, model mappings, output budgets and tools are not
changed. Raw conversation capture still records the unmodified incoming request.

This is a compatibility workaround, not a claim that the upstream quota is
exhausted. It **lowers the instruction priority of the matched default template**.
Do not use it when that template must remain a system-level policy. Account-level
opt-in avoids changing other customers or native Anthropic/Gemini accounts.

In a production reproduction on 2026-11-18, the captured DeepSeek Harness request
used `gemini-4-carbon-high`, 51 function tools and a 128000-token output budget.
With a 256-token verification budget, the original system template received an
upstream 429 and was then cooled down, causing follow-up routing 503s. The same
complete input and tools, with only that template's role changed to user context,
returned HTTP 200 and `response.completed`. Same-length neutral system text and
short inputs with the original tools also worked. The upstream's precise reason
for rejecting this template remains unknown; model-name aliases, quota exhaustion
and output-budget-only explanations are not established by these observations.

Use the existing account update API to merge the boolean into `extra`; preserve
all other fields and let the normal scheduler publication refresh the account.
Disable it by setting the boolean to `false`. No database migration is required.
This option does not enable AI Credits, change model quality or execute tools.
