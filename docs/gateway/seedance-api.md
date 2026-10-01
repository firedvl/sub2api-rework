# Seedance Tasks

The Gateway supports Ark's native asynchronous video task API on opted-in OpenAI API-key accounts. Configure a custom API base such as `https://ark.cn-beijing.volces.com/api/v3`, enable the **Seedance (Ark)** endpoint capability, and add an explicit model mapping. No model IDs or prices are added to discovery by enabling this capability.

Standard-mode keys use OpenAI or Composite groups with media generation permission. Simple mode also accepts ungrouped keys; account endpoint capability and credential checks still apply.

## Routes

- `POST /api/v3/contents/generations/tasks` creates a task from a JSON object with `model` and nonempty `content`.
- `GET /api/v3/contents/generations/tasks/:task_id` reads a task.
- `DELETE /api/v3/contents/generations/tasks/:task_id` deletes a task without an automatic refund.

The same routes are available under `/v3`, `/v1`, and no version prefix. There is no task-list route. Use the native ID from the create response for reads and deletes.

Content items and extension fields remain unchanged; only the model follows configured account mapping. Responses keep the native provider envelope. Invalid task IDs or malformed completion usage fail explicitly. Creates do not redirect or automatically retry because an uncertain failure may have already created a billable task.

## Ownership And Billing

Task bindings isolate user, API key, group and provider namespace. Reads remain on the submitting account, with current account/group/model eligibility checks. Missing ownership or create-time model metadata fails closed. Redis must retain the task binding and billing snapshot until completion; the default pending-billing TTL is 24 hours.

Create requests do not charge token usage. A successful completion read uses the provider's nonnegative integer `usage.completion_tokens` and the configured output-token price, not Grok's per-second tariff. Pending metadata preserves the create-time billing model and public alias. Repeated reads share the existing claim and durable usage dedup key; a failed billing write releases the claim for a later read.

There is no background task polling or callback settlement. A task that is never queried after completion is not automatically charged. Queued, running and failed tasks do not charge completion tokens.

Protocol fields follow the [official Ark Go SDK](https://github.com/volcengine/volcengine-go-sdk/blob/master/service/arkruntime/model/content_generation.go). Verification uses injected local fixtures, not live video generation.
