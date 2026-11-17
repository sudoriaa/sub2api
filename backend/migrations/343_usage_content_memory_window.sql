-- Conversation bodies are now a volatile per-process window. Discard ONLY
-- the previous temporary 24-hour payload tables; permanent usage and billing
-- tables are deliberately untouched. No new conversation bodies reach SQL.
DROP TABLE IF EXISTS usage_request_contents;
DROP TABLE IF EXISTS usage_content_budget;
