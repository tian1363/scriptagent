# Design QA: persistent chat composer

- Source visual truth: `/var/folders/y9/8873k5w913b1ty4k529xrsw40000gn/T/TemporaryItems/NSIRD_screencaptureui_zQ72IH/截屏2026-08-26 22.01.39.png`
- Implementation capture: in-app Browser tab 14, viewport capture after commit working tree changes
- Viewport: 1536 × 900 CSS px, device scale factor 1
- Source pixels: 2048 × 1119, including browser chrome
- Implementation pixels: 1536 × 900, app viewport only
- State: desktop chat, long historical assistant response selected, normal browser zoom

## Full-view comparison evidence

The source shows the long response expanding the main grid beyond the viewport, with the composer entirely below the fold. In the revised implementation the app frame ends at 900 px, the workspace occupies 832 px below the 68 px top bar, the chat pane occupies 784 px, and the composer remains visible from y=705 to y=858. Only the message list scrolls (`clientHeight: 543`, `scrollHeight: 1784`).

## Focused region evidence

The lower chat region was inspected at normal zoom. The composer textarea, skill action, asset action, add button, and send button are all visible. A separate focused crop was unnecessary because these controls are legible in the full viewport capture and their bounding boxes were measured directly.

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
