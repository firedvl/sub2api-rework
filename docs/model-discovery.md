# Automatic model discovery

The server refreshes model inventories on startup and every hour for enabled,
schedulable provider accounts. Accounts synced within the last hour are skipped.
It uses the same authenticated discovery code as the admin **Sync upstream models**
action. No inference requests are sent by this job.

New IDs appear in the gateway catalog and the admin connection test picker.
Provider pagination is followed within the existing response size limit. Failed
or invalid responses keep the last saved inventory and retry on the next cycle.
Capability metadata comes from the upstream, with models.dev filling missing
fields where the provider matches. Existing pricing refresh remains separate.

Account mappings, group allowlists, custom model lists, and pinned Codex manifests
still control which models clients see. Explicit restrictions are never expanded
by discovery. Unrestricted accounts can expose new provider models automatically.
Internal Claude routes and restricted Mythos IDs stay out of the public picker.

Discovery requires a supported provider model endpoint. Bedrock, service-account
formats, and Gemini Code Assist accounts without model listing retain their
existing catalog behavior. A model appearing in the provider's list does not
grant an account permission or credits to run it.

For Claude Code, enable gateway discovery in user settings, then restart the
session:

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
  }
}
```

Keep existing settings and credentials when adding this key. See the official
[gateway discovery instructions](https://code.claude.com/docs/en/llm-gateway).
