# Design QA: persistent chat composer

> Updated: 2026-08-28 · This is a regression record, not the complete product specification.

Additional verified baselines after the original composer fix:

- Main navigation defaults to a 68 px icon rail and expands to 224 px.
- Collapsed chat history uses an inline 36 px control in the context bar; it does not overlap the message viewport.
- Chat composer uses the coral brand family for background, border, focus ring and send action.
- Product knowledge and other top-level workspaces align their page header at 48 px from the viewport top.
- Only the conversation message region scrolls; global navigation and composer remain fixed in the app frame.

- Source visual truth: `/var/folders/y9/8873k5w913b1ty4k529xrsw40000gn/T/TemporaryItems/NSIRD_screencaptureui_zQ72IH/截屏2026-08-26 22.01.39.png`
- Implementation capture: in-app Browser tab 14, viewport capture after commit working tree changes
- Viewport: 1536 × 900 CSS px, device scale factor 1
- Source pixels: 2048 × 1119, including browser chrome
- Implementation pixels: 1536 × 900, app viewport only
- State: desktop chat, long historical assistant response selected, normal browser zoom

## Full-view comparison evidence

The source shows the long response expanding the main grid beyond the viewport, with the composer entirely below the fold. The original fix constrained the app frame and kept the composer visible; the current shell has no redundant global top bar and uses the full viewport. Only the message list scrolls.

## Focused region evidence

The lower chat region was inspected at normal zoom. The composer textarea, skill action, single asset-upload action, and send button are visible. The redundant plus action has since been removed.

## Required fidelity surfaces

- Fonts and typography: unchanged from the existing product design; normal zoom remains readable and no text scaling is required.
- Spacing and layout rhythm: fixed. Header, context bar, scrollable messages, and composer now fit within one viewport without overlap.
- Colors and visual tokens: unchanged.
- Image quality and asset fidelity: unchanged; no image assets were introduced or replaced.
- Copy and content: unchanged.

## Findings and comparison history

1. P0 before fix: the composer bottom was at y=1307 in a 900 px viewport, blocking the primary chat action.
2. First fix: constrained the app frame and chat surface, but the workspace implicit grid row still expanded to 1281 px.
3. Final fix: set the workspace row to `minmax(0, 1fr)`, allowed the main pane to shrink, and scoped overflow to the message list. Post-fix composer bottom is y=858 and the workspace bottom is y=900.

## Primary interactions and runtime checks

- Opened History and selected the long “生成15s广告脚本” conversation.
- Verified the composer is visible at 100% zoom.
- Verified the long message list remains independently scrollable.
- Browser console errors: none.

final result: passed

## Image-reference prompt interaction

> Checked: 2026-09-06

- Target: video prompt image-reference interaction shown in the supplied screenshot.
- Build: passed.
- Source image: available and reviewed.
- Prototype capture: blocked because the in-app browser could not verify its admin-enforced security policy for the local URL.
- Interaction verification: blocked for the same reason; no browser security control was bypassed.

final result: blocked

---

# Product research workspace design QA

## Scope

- Reference: `/Users/bluething/.codex/generated_images/01a03941-4243-7231-9ce7-8285720105f6/exec-86ffc50c-735d-4291-9137-b2c9d84639a2.png`
- Implementation capture: `/tmp/scriptagent-product-qa-initial.png`
- Side-by-side comparison: `/tmp/scriptagent-design-compare.png`
- Viewport: 1440 × 1024 CSS pixels, device scale factor 1
- State: authenticated product workspace, first product selected, latest creative report selected, product-data drawer open

## Visual evidence

The full-page capture shows the intended report-first hierarchy: compact product library on the left, product identity and creative strategy report in the center, and progressively disclosed product source material on the right. The reference and implementation were normalized to the same pixel dimensions and reviewed side by side.

Focused checks covered the selected product row, product header, context line, report heading and summary, clamped Markdown report, primary and secondary report actions, report history menu, document disclosure, asset strip, selected asset preview, file metadata, and drawer controls.

## Findings

- P0: none.
- P1: none.
- P2: none.
- Accepted implementation differences: the app preserves its existing collapsed global navigation preference; the QA fixture uses three real repository assets rather than five generated placeholders; the report body reflects backend-supported Markdown rather than mock-only structured fields.
- No horizontal or page-level vertical overflow was present at the target viewport.
- No console exceptions, warnings, or failed UI-state assertions were observed.

## Interaction verification

- Switch product: passed.
- Open report history and expose both stored versions: passed.
- Expand the complete report: passed.
- Open and cancel report regeneration: passed.
- Open and close the selected asset preview: passed.
- Collapse and reopen the product-data drawer: passed.
- Continue analysis with the current product attached and draft populated: passed.

## Regression verification

- Production frontend build: passed.
- Frontend unit tests: 15 passed, 0 failed.
- Browser checks used backend-shaped product, report, Markdown, and asset payloads. The fixture server was temporary and did not mutate application data.

## Iteration history

1. Replaced the previous dense product-management page with a report-first three-column workspace.
2. Reduced always-visible controls and moved edit/start actions, source configuration, full report content, and document content behind progressive disclosure.
3. Added responsive behavior, real asset thumbnails, drawer controls, report history, and verified all supported interactions.

final result: passed
