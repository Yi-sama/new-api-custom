# Image generation tab

- Objective: add a logged-in image-generation workspace to the existing Playground.
- Scope: add `/pg/images/generations`, reuse the existing OpenAI image relay/adapters and billing path, and add a chat/image tab.
- User-visible design: image generation is a conversation. Prompts appear as user messages and generated images appear together in the assistant reply. The toolbar supports importing, exporting, and clearing an image conversation.
- Parameters: the UI only exposes image count, size, quality, and style. Response format is fixed internally to URL. Moderation, input fidelity, vendor JSON, watermark, and other provider-facing fields are not shown.
- Model filtering: `/api/user/models?modality=image` returns only known image-generation models or models explicitly configured for the image-generation endpoint. The frontend clears the previous modality's model list while switching tabs, so chat models are never left selectable during the image-model request. A controller regression test confirms that `gpt-image-1` is returned while `gpt-4o` is excluded.
- Verification: root `go build ./...`; targeted controller model-filter tests; relay constant, middleware, and router tests; frontend typecheck; affected-file oxlint; payload Vitest (3 passed); strict UTF-8 checks; and production Rsbuild build all passed.
- English and Chinese translations were added for the conversation actions, empty state, settings, and no-image-model state.
- Local deployment: rebuilt `bin/new-api-local-b-c.exe` and restarted `http://localhost:3000` with the existing `.local-tests/abc-shared.db`. `/api/status` and the image conversation chunk both returned 200; stderr was empty. Backups: `bin/new-api-local-b-c.exe.pre-image-chat-20260923` and `bin/new-api-local-b-c.exe.pre-image-chat-model-clear-20260923`.
- Manual validation: pending visual confirmation in an already logged-in browser and a real generation request with an enabled image-generation channel.
- Known ceiling: this first pass does not include multipart image editing, multi-image upload, or SSE partial-image rendering.
