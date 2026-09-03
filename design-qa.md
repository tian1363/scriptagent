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
