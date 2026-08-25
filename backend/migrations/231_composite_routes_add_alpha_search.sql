-- Allow Composite model routes to scope Codex standalone web search requests.
ALTER TABLE composite_model_routes
    DROP CONSTRAINT IF EXISTS composite_model_routes_endpoint_check;

ALTER TABLE composite_model_routes
    ADD CONSTRAINT composite_model_routes_endpoint_check
    CHECK (endpoint IN ('any', 'messages', 'count_tokens', 'responses', 'alpha_search',
                        'chat_completions', 'embeddings', 'images', 'gemini'));
