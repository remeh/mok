# Plan: TUI Navigation & Rendering Overhaul

## Goal

Make scrolling, expand/collapse, and general navigation in the TUI **predictable, reliable, and snappy**. Today the TUI feels sluggish and error-prone: scroll position drifts during streaming, the user sometimes can't reach the true bottom, the input area disappears, and click targets shift under the cursor.

This plan identifies root causes and proposes a refactor that fixes them at the source rather than patching symptoms.

---

## Root Causes (the "why it feels broken" inventory)

### RC1 — Two diverging line-count implementations

`MessageView` has **two ways of counting lines per message** that must agree but don't:

- `messageLineCount(msg)` — used by `IsAtBottom()`, `ScrollUp/Down/Page*`, `ScrollToBottom()`, and (indirectly) `Render()`'s clamp. It counts lines from the **raw** content via `wordwrap.String(content, width-2)`.
- `renderMessage(msg)` — the actual renderer. For assistant messages it pipes content through `mdRenderer.Render()` (Glamour markdown) **before** wordwrap, which dramatically changes line counts (code fences, headings, bullets, ANSI styling all reflow differently).

**Symptom:** scroll math thinks the document is N lines but it actually rendered M lines. `ScrollToBottom()` lands at "estimated bottom", which can be many lines short of (or past) the real bottom. The user "can't scroll to the bottom" because the scrollPos clamp uses one number while the actual viewport content is sized by another.

`message_view.go:124-181` (count) vs `message_view.go:300-414` (render).

### RC2 — Streaming cursor adds and removes a line every 4 ticks

`renderMessage()` appends `"▌"` half the time and `""` the other half (lines 403–411), based on `cursorFrame % 8 < 4`. `messageLineCount()` doesn't model this at all.

**Symptom:** during streaming `len(allLines)` (used for auto-scroll and clamp) oscillates by ±1 every 400ms while `totalLineCount()` (used for `IsAtBottom`) stays constant. They disagree on which frame the user is "at the bottom", so the scroll indicator flickers and the autoscroll override fights manual scrolls.

### RC3 — Auto-scroll is overridden inside `Render()`

`Render()` does `if v.autoScroll { v.scrollPos = max(0, len(allLines)-v.visible) }` (line 277). This is a side-effect from a renderer, and it runs on *every* frame at 10 Hz. Combined with RC2, the scroll position is rewritten constantly during streaming.

`AddMessage()` unconditionally sets `autoScroll = true` (line 50), so any new tool result, thinking delta, or assistant message yanks the user back to the bottom even if they were reading scrollback.

### RC4 — `ScrollDown()` doesn't symmetrise with `ScrollUp()`

`ScrollUp()` sets `autoScroll = false`; `ScrollDown()` doesn't touch it. Mouse-wheel scroll-down therefore doesn't transition the state machine, so once the user has scrolled up and back down, the next streamed message still won't snap to bottom (or worse, snaps inconsistently depending on what arrived in between).

### RC5 — Layout mutates during `Render()`

`Screen.adjustMessageViewHeight()` calls `s.msgView.SetDimensions(...)` from inside `Render()` (`screen.go:50-65`). `SetDimensions` nukes `mdRenderer = nil`, so every frame where the user toggles in/out of scrolled-up state rebuilds the Glamour renderer (allocations, regex compilation). It also changes `v.visible` mid-render, which retroactively invalidates `IsAtBottom()` calls that already happened earlier in the same tick.

This is the source of the perceived sluggishness: scrolling near the boundary thrashes the markdown renderer.

### RC6 — Arrow keys are routed by a fragile heuristic

In `app.go:156-184`, Up/Down/PgUp/PgDn/Home/End scroll **only if** `Value() == "" || IsScrolledUp()`. The moment the user has typed anything, arrow keys steal focus to history navigation; they have no way to scroll without first clearing the input.

This is the single biggest "navigation feels error-prone" complaint: the same key does two different things based on a hidden state.

### RC7 — Click hit-test uses stale layout assumptions

`app.go:214` checks `if msg.Y < m.height-2` to decide whether the click landed in the message area. But when scrolled up, the layout is `msgView (h-3) + indicator (1) + input (1) + status (1)`. A click on the indicator row (`y == h-3`) passes the check and is fed to `MessageAtY(y)`, toggling whichever message happens to fall there. Clicks just above the input therefore behave randomly when the indicator is visible.

### RC8 — `MessageAtY` mapping uses `lineRanges` built from `messageLineCount`-shaped data

`lineRanges` is rebuilt during `Render()` from `len(lines)` returned by `renderMessage()`. That part is internally consistent. But because `renderMessage` includes the streaming cursor line and `messageLineCount` doesn't, the ranges shift every 400ms. Clicks therefore land on different messages depending on cursor blink phase.

### RC9 — No keyboard affordance for expand/collapse

Toggle is mouse-only (`tea.MouseButtonLeft` in `app.go:208-225`). Users on terminals without mouse passthrough, or doing keyboard-only navigation, have no way to expand a tool result. `Ctrl-O` (expand all) is the only escape hatch, and it's all-or-nothing.

### RC10 — No persistent "user has taken control" flag

The state machine has only `autoScroll bool`, which is implicitly recomputed from heuristics. There's no notion of *intent*: "the user explicitly chose this scroll position; don't move it until they snap back to the bottom". This is what real chat UIs (iMessage, Slack, terminals) get right — once you scroll up, new content is queued silently until you scroll back to the bottom.

---

## Design Principles for the Overhaul

1. **One renderer, one count.** The number of lines a message occupies comes from the renderer, never re-estimated. If markdown rendering is going to add 30 lines, the scroll math sees 30.
2. **Render is pure.** `Render()` doesn't mutate scroll state, dimensions, or anything else. State mutations happen only in `Update()` handlers.
3. **Streaming doesn't change layout.** The blinking cursor is an in-line glyph, not an extra line. Or: the cursor occupies a *reserved* line that exists whether visible or not.
4. **Explicit user intent.** Track `userPinnedScroll bool`. Set it true when the user issues any scroll-up gesture. Clear it when the user scrolls back to the true bottom (within a 1-line tolerance) or hits End.
5. **Layout is fixed at `SetDimensions` time.** No mid-render resizing. Indicator overlays the message view (or moves to the status bar) instead of stealing a row.
6. **Keys do one thing.** Scroll has its own bindings (PgUp/PgDn, Ctrl-U/Ctrl-D, Shift+Up/Down, mouse wheel). Up/Down always go to history when input has focus. No "guess by emptiness" routing.
7. **Every click is layout-aware.** Hit-testing asks the screen "what region is this Y in?" rather than hardcoding offsets.

---

## Proposed Architecture

### A. `MessageView` becomes a render-cache + scroll model

```go
type renderedMessage struct {
    msg       *types.Message
    lines     []string  // pre-styled, post-markdown, post-wordwrap
    fingerprint string  // hash of (Type, Content, Streaming, Collapsed, ThinkingExpanded, width)
}

type MessageView struct {
    theme      Theme
    messages   []*types.Message
    rendered   []renderedMessage  // parallel to messages, but cached
    width      int
    height     int                // viewport height in lines
    scrollPos  int                // top visible line, 0 = top of doc
    pinned     bool               // user has taken explicit control of scroll
    cursorFrame int
    mdRenderer *markdownRenderer
}
```

**Cache invalidation:** before each `Render()`, walk `messages` and re-render only those whose fingerprint has changed (or whose width changed since last render). Streaming messages always re-render (their content grows), but we exclude the cursor glyph from the cache.

**Streaming cursor:** appended to the *last line of the streaming message in place*, not as a separate line. `▌` is inserted at the end of the final wrapped line. This way the line count is stable across blink frames.

**Single source of truth for line count:**
```go
func (v *MessageView) totalLines() int {
    n := 0
    for _, r := range v.rendered { n += len(r.lines) }
    return n
}
```
`messageLineCount()` is deleted. Anywhere it was used (clamping, page math, IsAtBottom) calls into the cache.

### B. `Screen` layout is fixed; indicator overlays

Two viable options — pick one:

**Option B1 (preferred): scroll indicator lives in the status bar.**
The status bar already shows model + tokens + state. Add a "↓ N lines below" segment that appears when `pinned && !atBottom`. No layout change ever happens between scrolled and unscrolled state. Click target stability is preserved. Removes `adjustMessageViewHeight` entirely.

**Option B2: indicator overlays the bottom row of the message view.**
The msgView still claims `h-2` rows but `Render()` overwrites the last row with the indicator string when scrolled up. The user loses one line of *content* (the bottom-most line), not a line of layout. No re-dimensioning, no `mdRenderer` reset.

I recommend **B1** — it's simpler, more discoverable, and removes a whole class of layout bugs. B2 is a fallback if status-bar real estate is too tight.

### C. Scroll state machine

```go
// Called by every scroll-up gesture (arrow up, PgUp, mouse wheel up, Home, Ctrl-U).
func (v *MessageView) ScrollUp(n int) {
    v.scrollPos = max(0, v.scrollPos - n)
    v.pinned = true
}

// Called by every scroll-down gesture.
func (v *MessageView) ScrollDown(n int) {
    maxScroll := max(0, v.totalLines() - v.height)
    v.scrollPos = min(maxScroll, v.scrollPos + n)
    if v.scrollPos >= maxScroll {  // landed at true bottom
        v.pinned = false
    }
}

// Called when a new message arrives or the streaming message grows.
func (v *MessageView) maybeFollowTail() {
    if v.pinned { return }
    v.scrollPos = max(0, v.totalLines() - v.height)
}
```

`AddMessage` calls `maybeFollowTail`. **No mutation in `Render()`.** The `autoScroll` field is replaced by `pinned` (inverted semantics, clearer intent).

`ScrollToBottom()` (End key, new prompt submitted) explicitly sets `v.pinned = false` and calls `maybeFollowTail`.

### D. Key routing

Replace the `Value() == "" || IsScrolledUp()` heuristic with explicit bindings:

| Key | Action when input focused | Action when input not focused (agent running) |
|---|---|---|
| Up / Down | History prev/next | Scroll up/down 1 line |
| PgUp / PgDn | Scroll up/down 1 page | Scroll up/down 1 page |
| Ctrl-U / Ctrl-D | Scroll up/down 1 page | Scroll up/down 1 page |
| Home / End | Cursor home/end *in input* | Scroll to top / bottom |
| Shift+Up / Shift+Down | Scroll up/down 1 line | (same) |
| Mouse wheel | Scroll | Scroll |
| `Ctrl-Home` / `Ctrl-End` | Scroll to top/bottom of msgView | (same) |

Concretely: **PgUp/PgDn always scroll, never do anything to input.** That's a stable, reliable affordance the user can rely on regardless of input state. Up/Down for scroll requires either Shift modifier or empty input, but PgUp/PgDn and Ctrl-U/Ctrl-D are unambiguous.

### E. Layout-aware click hit-testing

`Screen` exposes:
```go
type Region int
const (
    RegionMessageView Region = iota
    RegionInput
    RegionStatus
    RegionScrollIndicator  // only if Option B2 is chosen
)

func (s *Screen) RegionAt(x, y int) Region { ... }
```

`app.go` uses `Screen.RegionAt(msg.X, msg.Y)` to decide whether a click is for expand/collapse. This is the only correct way once the layout has any conditional rows.

### F. Keyboard expand/collapse

Add a "focused message" cursor:
- `Tab` / `Shift+Tab` move the focus marker through messages that have toggle-able state (tool results with summaries, thinking blocks).
- `Space` or `Enter` (when input is empty) toggles the focused message.
- The focused message gets a subtle visual marker (e.g. a `▶` in the gutter).

Optional, but it removes the mouse-only constraint and gives a deterministic way to expand specific results without the all-or-nothing `Ctrl-O`.

If the focus-cursor work is too large for one pass, the **minimum viable** improvement is: `Ctrl-O` toggles "all expanded" vs "all collapsed" instead of one-way expanding.

### G. Multiline input visibility (related navigation issue)

`InputArea.Render()` only shows one line, so Alt+Enter newlines vanish from view. Either:
- Render the input as N rows when it contains newlines (and reduce `msgView` height accordingly via `SetDimensions`, *not* during `Render`), or
- Render a single-line summary like `"line 1 ... [+3 more lines]"`.

This isn't strictly a scroll bug, but it's part of the "navigation feels unreliable" cluster — users can't see what they typed.

---

## Implementation Phases

Each phase is independently shippable and testable.

### Phase 1 — Single source of truth for line counts (highest ROI)

Files: `internal/tui/message_view.go`

1. Add `rendered []renderedMessage` cache and fingerprinting.
2. Move all `wordwrap` + markdown rendering into the cache builder.
3. Replace `messageLineCount` callers with `len(v.rendered[i].lines)`.
4. Delete `messageLineCount` (and its duplicated `wordwrap` calls).
5. Make streaming cursor an in-line glyph appended to the last wrapped line of the streaming message — never a separate line.

**Validation:** add a property test asserting `totalLines() == len(strings.Split(strings.Join(allRenderedLines, "\n"), "\n"))` for a corpus including markdown-heavy assistant messages, tool calls, tool results (collapsed and expanded), and a streaming message across 8 cursor frames.

After Phase 1: "can't scroll to the bottom" is gone.

### Phase 2 — Pure `Render()` and `pinned` state machine

Files: `internal/tui/message_view.go`, `internal/app/app.go`

1. Rename `autoScroll` → `pinned` with inverted semantics.
2. Remove the `if v.autoScroll { v.scrollPos = ... }` block from `Render()`.
3. Add `maybeFollowTail()`; call it from `AddMessage` and from a new `MessageGrew()` method called after each text/thinking delta.
4. Make `ScrollDown()` clear `pinned` when reaching the true bottom; `ScrollUp/PageUp/Home` set `pinned = true`.
5. `app.go` calls `MessageGrew()` from `EventTextDelta` / `EventThinkingDelta`.

**Validation:** test that streaming + manual scroll-up keeps the user pinned; scrolling back to the bottom unpins; new content auto-follows again.

After Phase 2: streaming no longer fights manual scroll; flicker is gone.

### Phase 3 — Move scroll indicator to status bar; remove `adjustMessageViewHeight`

Files: `internal/tui/screen.go`, `internal/tui/statusbar.go`

1. Delete `adjustMessageViewHeight` and the conditional indicator row.
2. Add `StatusBar.SetScrollHint(linesBelow int)`; render as `↓ N` segment when nonzero.
3. `Screen.Render()` is now: `msgLines + "\n" + input + "\n" + statusBar`. Always 3 sections, fixed heights, no conditional resizing.
4. Drop the `RenderScrollIndicator` method (or keep for tests).

After Phase 3: no more layout thrash; markdown renderer is built once per width change.

### Phase 4 — Key routing cleanup

Files: `internal/app/app.go`

1. Remove the `Value() == ""` test for PgUp/PgDn/Ctrl-U/Ctrl-D/Home/End scroll routing — those keys *always* scroll the msgView (Home/End remain input-cursor when input is focused; map Ctrl-Home/Ctrl-End to msgView for explicitness).
2. Up/Down: keep current behavior (history when input non-empty, scroll otherwise) but also add Shift+Up/Down → always scroll.
3. Add a `bindings.go` constants file documenting the bindings, for the future help screen.

After Phase 4: navigation feels deterministic; muscle memory works.

### Phase 5 — Layout-aware click hit-testing

Files: `internal/tui/screen.go`, `internal/app/app.go`

1. Add `Screen.RegionAt(x, y) Region`.
2. `app.go` mouse handler asks the screen rather than computing `m.height-2`.
3. `MessageView.MessageAtY` takes a clean Y already known to be in the msgView region.

After Phase 5: click-to-expand stops triggering on the indicator/status rows.

### Phase 6 — Keyboard expand/collapse (optional but recommended)

Files: `internal/tui/message_view.go`, `internal/app/app.go`, `internal/types/message.go`

1. Add `MessageView.focusedIdx int` (-1 = none).
2. Keys: `Tab`/`Shift+Tab` to cycle through toggle-able messages; `Space`/`Enter` toggles when input is empty.
3. Visual: focused message rendered with a `▶` gutter or accent border.
4. `Ctrl-O` becomes "toggle all" (expand if any collapsed, else collapse all).

### Phase 7 — Multiline input rendering

Files: `internal/tui/input.go`, `internal/tui/screen.go`

1. `InputArea.Render()` returns N lines based on content.
2. `Screen.SetDimensions` reserves `1 + extraInputLines + 1` for input+status.
3. Re-call `SetDimensions` from `Update()` (not `Render()`) when input height changes.

---

## Testing Strategy

### Unit tests (must pass before merging each phase)

- `totalLines()` equals actual rendered line count for every message type, with markdown content, with streaming, across width values 40/80/120.
- `IsAtBottom()` is monotonic with `scrollPos`: scrolling down never makes it false; scrolling up never makes it true.
- `pinned` invariants:
  - false after `NewMessageView`
  - true after any `ScrollUp/ScrollPageUp/ScrollToTop`
  - false after `ScrollToBottom` or after `ScrollDown` reaches the true bottom
  - never changes inside `Render()`
- `MessageAtY` returns stable indices across cursor blink frames during streaming.
- Cache fingerprint changes when and only when a visible property of the message changes.

### Integration / interactive tests

A scripted bubbletea test that:
1. Adds 50 messages of varied types (some with markdown, some tool results, one streaming).
2. Asserts `ScrollToBottom` puts the last line of the last message in the bottom viewport row.
3. Sends 100 cursor-tick messages while streaming; asserts `scrollPos` doesn't change.
4. Scrolls up 5 lines, then sends a new message; asserts `scrollPos` is unchanged (pinned).
5. Scrolls down to the bottom; asserts `pinned` clears; new message auto-follows.

### Manual smoke checklist

- [ ] Long markdown response — `End` lands at the true bottom; no whitespace gap.
- [ ] Stream a multi-second response, scroll up while streaming; stays where you left it.
- [ ] Scroll down to the bottom during streaming; auto-follow resumes.
- [ ] Click the row above the input bar while scrolled up; nothing toggles (it's the indicator/status, not a message).
- [ ] PgUp/PgDn always scroll, even mid-typing.
- [ ] Tab+Space (Phase 6) expands a tool result without touching the mouse.

---

## Files Touched

| File | Phases | Nature of change |
|---|---|---|
| `internal/tui/message_view.go` | 1, 2, 6 | Cache, pure render, focused index |
| `internal/tui/screen.go` | 3, 5 | Remove `adjustMessageViewHeight`, add `RegionAt` |
| `internal/tui/statusbar.go` | 3 | Add scroll hint segment |
| `internal/tui/input.go` | 7 | Multiline render |
| `internal/tui/theme.go` | 6 | Optional accent for focused message |
| `internal/app/app.go` | 2, 4, 5, 6 | `MessageGrew()` calls, key routing, region-aware clicks |
| `internal/tui/message_view_test.go` | 1, 2 | Cache + state machine tests |
| `internal/tui/screen_test.go` | 3, 5 | Region tests |

---

## Risk & Rollback

- **Phase 1** is the largest behavioral change and will likely catch test regressions in tool result and markdown rendering. Land it behind a code-path comparison: temporarily keep both `messageLineCount` and the cache, log when they disagree, fix the divergences before deleting the old path.
- Each phase is independently revertable. Phases 1–3 are mandatory; 4–7 are quality-of-life and can be sequenced based on user feedback.
- The `SCROLLING_BUG_ANALYSIS.md` and `PLAN-scroll-indicator.md` documents become obsolete after Phase 3 lands and should be deleted then.

---

## Out of Scope

- Search-in-scrollback (would be nice, but a separate feature).
- Copy-to-clipboard from the message view.
- Resizable panes / sidebar.
- Reflow on terminal resize is already handled by `SetDimensions` + cache invalidation in Phase 1; no extra work needed.
