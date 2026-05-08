# Plan: TUI Improvement Phases 4–7

This document is **self-contained**. Phases 1–3 of the original `PLAN-improve-TUI.md` have shipped; this document carries forward only what 4–7 need to know.

## What you can rely on (already done in Phases 1–3)

These invariants hold in `internal/tui` today:

- **Single source of truth for line counts.** `MessageView` keeps a `rendered []renderedMessage` cache (keyed by a fingerprint of width + visible message state). `totalLineCount()` sums `len(rendered.lines)`. The legacy `messageLineCount()` estimator is gone. Streaming messages always re-render; everything else only on fingerprint change.
- **Streaming cursor is in-line.** The blinking `▌` is appended to the last wrapped line of the streaming message. Line count is constant across blink frames.
- **`Render()` is pure.** It does not mutate `scrollPos` or `pinned`. Defensive max-clamp only.
- **Explicit `pinned` state machine.** `pinned == false` means "follow tail" (default); `pinned == true` means "user took control". Set by any `ScrollUp/PageUp/ToTop`, cleared by `ScrollDown/PageDown` reaching the true bottom or by `ScrollToBottom`. Submitting a prompt calls `ScrollToBottom`.
- **Tail-follow on growth.** `MessageView.AddMessage` and `MessageView.MessageGrew()` both call `maybeFollowTail()`, which is a no-op when pinned. `app.go` calls `MessageGrew()` from every agent event that grows or mutates a message: `EventMessageStart`, `EventTextDelta`, `EventThinkingDelta`, `EventToolCallStart`, `EventToolCallUpdate`, `EventToolCallEnd`, `EventToolResult`, `EventError`. Click and Ctrl-O toggles also call it.
- **Fixed three-row layout.** `Screen.Render()` always emits `msgView (h-2) + input (1) + status (1)`. There is no conditional indicator row; the scroll-position hint lives inside the status bar as a `↓N` segment driven by `MessageView.LinesBelow()` and `StatusBar.SetScrollHint(n)`.
- **`adjustMessageViewHeight` is gone.** Markdown renderer is rebuilt only on real width changes.
- **`IsScrolledUp` guards on input keys are gone.** Typing and submission work even when the user has scrolled up — submission snaps to the bottom via `ScrollToBottom`.

If any of those is no longer true when you start, fix that first — these phases assume them.

## What still hurts (the 4 remaining root causes)

Lifted from the original analysis, restated in current terms:

- **RC6 — Up/Down routing by `Value() == ""` heuristic.** `app.go:155-184` decides whether arrow / Page / Home / End scroll the view or go to the input area based on whether the input is empty *or* the user is scrolled up. Same key does two things based on hidden state. Once the user has typed anything, they cannot scroll with arrow keys at all — they must clear the input first. This is the single biggest "navigation feels error-prone" complaint.
- **RC7 — Mouse hit-test uses hardcoded `m.height-2`.** `app.go` (in the `tea.MouseButtonLeft` case) checks `if msg.Y < m.height-2`. That cutoff is correct for the current Phase-3 layout (msgView occupies rows 0..h-3), but it's still hardcoded — any future layout change will silently misroute clicks. There's no "what region is this?" abstraction.
- **RC9 — Expand/collapse is mouse-only.** Toggling a tool result requires a click on the right row. Keyboard-only users can only "expand all" via `Ctrl-O`. There's no way to expand a single specific tool result without the mouse.
- **G — Multiline input is invisible.** `Alt+Enter` inserts a newline into the input value, but `InputArea.Render()` only draws one row. Users can't see what they typed beyond the first line. They also can't tell where their cursor is across lines.

---

# Phase 4 — Key Routing Cleanup

## Goal

Make every navigation key do exactly one thing, regardless of input state.

## Current behavior (the bug)

```go
// internal/app/app.go (Update, KeyMsg branch)
case tea.KeyUp, tea.KeyDown:
    if m.Screen.GetInputArea().Value() == "" || m.Screen.IsScrolledUp() {
        // scroll
    } else {
        // history
    }

case tea.KeyPgUp, tea.KeyPgDown, tea.KeyCtrlU, tea.KeyCtrlD:
    if m.Screen.GetInputArea().Value() == "" || m.Screen.IsScrolledUp() {
        // scroll
    }
    // else: silently ignored

case tea.KeyHome, tea.KeyEnd:
    if m.Screen.GetInputArea().Value() == "" || m.Screen.IsScrolledUp() {
        // scroll to top/bottom
    }
    // else: silently ignored
```

Three problems: PgUp/PgDn/Home/End are silently ignored when input is non-empty and not scrolled up; arrow keys are stolen by history the moment you type; `Ctrl-U` is ambiguous (input.go uses it for "delete line" too).

## Target binding table

| Key | When input has focus | When agent is running (input not focused) |
|---|---|---|
| `Up` / `Down` | History prev / next (existing) | Scroll up / down 1 line |
| `Shift+Up` / `Shift+Down` | Scroll up / down 1 line | Scroll up / down 1 line |
| `PgUp` / `PgDn` | **Always scroll**, never touches input | Scroll page |
| `Ctrl-D` | **Always scroll page down** | Scroll page down |
| `Home` / `End` | Cursor home / end *in input* (existing input.go handler) | Scroll to top / bottom |
| `Ctrl-Home` / `Ctrl-End` | Scroll to top / bottom | Scroll to top / bottom |
| Mouse wheel | Scroll | Scroll |
| `Ctrl-U` | **Input only** — delete line (existing input.go behavior). Remove from scroll routing. |

The principle: **PgUp/PgDn are unambiguous scrolling.** Up/Down stay context-sensitive (history/scroll) for muscle memory but gain a `Shift+` variant that always scrolls.

## Implementation

### 1. Detect Shift modifier on Up/Down

Bubbletea v1.3.10 reports modifier state on `tea.KeyMsg` via `msg.Alt`. Shift is *not* directly exposed for arrow keys — they arrive as their own `tea.KeyType` constants (`tea.KeyShiftUp`, `tea.KeyShiftDown`) when the terminal sends them. Verify which constants exist:

```sh
grep -E "^\s+Key(Shift)?(Up|Down|PgUp|PgDown|Home|End)\b" \
  /Users/remy/go/pkg/mod/github.com/charmbracelet/bubbletea@v1.3.10/key.go
```

If `tea.KeyShiftUp` / `tea.KeyShiftDown` exist, use them directly. If not, fall back to `Up`/`Down` only and document the limitation. (Most modern terminals send `\e[1;2A` for Shift+Up, which bubbletea's parser maps to `KeyShiftUp` — should be fine.)

### 2. Rewrite the KeyMsg switch arms

Concrete diff for `internal/app/app.go`:

```go
case tea.KeyUp, tea.KeyDown:
    // Up/Down route to input history when the input has focus and the
    // user is at the bottom (default editing posture). Anywhere else,
    // they scroll. Use Shift+Up/Down to scroll without losing the input.
    inHistoryMode := !m.agentRunning && m.Screen.GetInputArea().Value() != ""
    if inHistoryMode {
        m.Screen.GetInputArea().HandleKey(msg.Type)
    } else {
        if msg.Type == tea.KeyUp {
            m.Screen.GetMessageView().ScrollUp()
        } else {
            m.Screen.GetMessageView().ScrollDown()
        }
    }

case tea.KeyShiftUp, tea.KeyShiftDown:
    // Always scroll, never history.
    if msg.Type == tea.KeyShiftUp {
        m.Screen.GetMessageView().ScrollUp()
    } else {
        m.Screen.GetMessageView().ScrollDown()
    }

case tea.KeyPgUp, tea.KeyPgDown:
    // Unconditional scroll. Removed Ctrl-U/Ctrl-D from this case so
    // Ctrl-U remains "delete line" in the input.
    if msg.Type == tea.KeyPgUp {
        m.Screen.GetMessageView().ScrollPageUp()
    } else {
        m.Screen.GetMessageView().ScrollPageDown()
    }

case tea.KeyCtrlD:
    // Ctrl-D always scrolls page down (mirrors readline/less semantics).
    // Note: this preempts EOF behavior; if EOF is needed elsewhere,
    // gate this on input being empty + agent not running.
    m.Screen.GetMessageView().ScrollPageDown()

case tea.KeyHome, tea.KeyEnd:
    // When input has focus, Home/End move the input cursor (delegated).
    // When agent is running (input not focused), they jump in scrollback.
    if !m.agentRunning && m.Screen.GetInputArea().Value() != "" {
        m.Screen.GetInputArea().HandleKey(msg.Type)
    } else {
        if msg.Type == tea.KeyHome {
            m.Screen.GetMessageView().ScrollToTop()
        } else {
            m.Screen.GetMessageView().ScrollToBottom()
        }
    }

case tea.KeyCtrlHome, tea.KeyCtrlEnd:  // if defined; else skip this case
    if msg.Type == tea.KeyCtrlHome {
        m.Screen.GetMessageView().ScrollToTop()
    } else {
        m.Screen.GetMessageView().ScrollToBottom()
    }
```

### 3. Remove the dual-purpose `Ctrl-U` from scroll routing

The `tea.KeyCtrlU` case in the existing `KeyPgUp/KeyPgDown/KeyCtrlU/KeyCtrlD` arm should be deleted. `input.go` already handles `Ctrl-U` as "delete line". Leaving `Ctrl-D` for scroll is fine because `input.go` does not handle `Ctrl-D` (verify with `grep KeyCtrlD internal/tui/input.go`).

### 4. Update `input.go` if `Ctrl-D` collides

If `input.go` `HandleKey` does anything with `tea.KeyCtrlD`, decide which wins. Recommended: remove it from input.go (no one expects Emacs-style forward-delete here) so `Ctrl-D` is unambiguously scroll-page-down.

### 5. Document bindings

Create `internal/tui/bindings.go` with a `var Bindings = []KeyBinding{...}` so a future `?` help screen has a single source of truth. Each `KeyBinding` is `{Key string, Description string, Context string}`. This is documentation, not behavior — the actual routing stays in `app.go`. Optional but recommended.

## Files

- `internal/app/app.go` — the switch rewrite.
- `internal/tui/input.go` — drop `Ctrl-D` if present.
- `internal/tui/bindings.go` — new, optional.

## Tests

In `internal/app/app_test.go` (create if missing) — these are end-to-end-ish tests that drive `Update()` directly:

- `TestPgUpScrollsRegardlessOfInput`: type "hello" into input, send `tea.KeyMsg{Type: tea.KeyPgUp}`, assert `Screen.GetMessageView().scrollPos` decreased and input value still equals "hello".
- `TestUpDownGoToHistoryWhenInputNonEmpty`: history setup with one prior message, type "x" into input, send `KeyUp`, assert input value changed to history entry (not scrolled).
- `TestShiftUpAlwaysScrolls`: type "hello", send `KeyShiftUp`, assert scroll moved and input still "hello".
- `TestCtrlUStillDeletesLine`: type "hello", send `KeyCtrlU`, assert input is "" and scroll did NOT move.

A pure-tui test for `MessageView` is already covered by Phase 2 invariants; nothing new needed there.

## Risks

- `Ctrl-D` in some terminal configs sends EOF/quit. If you have any test or user habit that relied on `Ctrl-D = quit`, document the change in the commit message and consider keeping `Ctrl-C` as the only quit path (it already is).
- If `tea.KeyShiftUp` / `tea.KeyShiftDown` aren't recognized on the user's terminal, the binding silently doesn't fire. Add a fallback note in the help text.

## Definition of done

- All four `Update()` switch arms above are rewritten as specified.
- The four new tests pass.
- Manual: `mok`, type a long prompt, press `PgUp` → scrolls. Press `End` (with input non-empty) → cursor jumps to end of input, does NOT scroll. Press `Up` (input non-empty) → history. Type more, press `Shift+Up` → scrolls without losing input.

---

# Phase 5 — Layout-aware Click Hit-Testing

## Goal

Replace the hardcoded `msg.Y < m.height-2` cutoff in mouse handling with an explicit "what region is this?" query owned by `Screen`. Ensures clicks on the input or status bar never accidentally toggle a message, and that future layout changes (e.g., a multi-row input from Phase 7) automatically work.

## Current behavior (the bug)

```go
// internal/app/app.go (Update, MouseMsg branch)
case tea.MouseButtonLeft:
    if msg.Action != tea.MouseActionPress {
        break
    }
    if msg.Y < m.height-2 {  // <-- hardcoded
        idx := m.Screen.GetMessageView().MessageAtY(msg.Y)
        ...
    }
```

The cutoff happens to be correct for the Phase-3 layout (msgView is rows `0..h-3`). It is **not** correct in the following cases:

- If Phase 7 adds multi-row input (e.g. 3 lines), msgView shrinks to `h-4` but the cutoff still says `h-2`. Clicks on input rows 1 and 2 toggle messages.
- If anyone ever adds a header row, banner, or modal, the cutoff is silently wrong.

`MessageAtY` itself is correct — it uses `lineRanges` populated from cached render. The problem is "is this Y inside the message view at all?".

## Design

Introduce a `Region` enum on `Screen` and a `RegionAt(x, y)` query.

```go
// internal/tui/screen.go

type Region int

const (
    RegionNone Region = iota
    RegionMessageView
    RegionInput
    RegionStatus
)

// RegionAt returns which screen region contains the given coordinate.
// Coordinates are 0-based, with (0,0) at the top-left.
func (s *Screen) RegionAt(x, y int) Region {
    if y < 0 || x < 0 || x >= s.width || y >= s.height {
        return RegionNone
    }
    msgViewBottom := s.msgView.height // msgView occupies rows [0, height)
    switch {
    case y < msgViewBottom:
        return RegionMessageView
    case y < msgViewBottom+1:  // one row of input (until Phase 7)
        return RegionInput
    case y < msgViewBottom+2:  // one row of status
        return RegionStatus
    default:
        return RegionNone
    }
}
```

Phase 7 will widen the input region; that's an isolated change to `RegionAt` then.

## Implementation

### 1. Add `Region` enum and `RegionAt` to `screen.go`

As above. Read `s.msgView.height` directly — it's already the source of truth for layout, set in `SetDimensions`. Don't re-derive from `s.height` unless you also subtract input + status, which makes RegionAt depend on layout details that should live in one place.

Optional: extract a `regionBounds` struct holding `(top, height)` for each region so RegionAt is a table walk. Overkill for 3 regions.

### 2. Rewrite the mouse handler in `app.go`

```go
case tea.MouseButtonLeft:
    if msg.Action != tea.MouseActionPress {
        break
    }
    if m.Screen.RegionAt(msg.X, msg.Y) != tui.RegionMessageView {
        break
    }
    idx := m.Screen.GetMessageView().MessageAtY(msg.Y)
    if idx >= 0 && idx < len(m.Messages) {
        msgAtClick := m.Messages[idx]
        switch {
        case msgAtClick.Type == types.MsgToolResult && msgAtClick.Summary != "":
            msgAtClick.Collapsed = !msgAtClick.Collapsed
            m.Screen.GetMessageView().MessageGrew()
        case msgAtClick.ThinkingText != "":
            msgAtClick.ThinkingExpanded = !msgAtClick.ThinkingExpanded
            m.Screen.GetMessageView().MessageGrew()
        }
    }
```

Note: `MessageAtY` takes a Y already in the message-view region; no offset translation is needed because msgView starts at row 0.

### 3. (Optional) Use `RegionAt` for wheel events too

Currently mouse wheel scrolls regardless of cursor position. That's fine. But if you wanted "scroll only when over msgView", gate `tea.MouseButtonWheelUp/Down` on `RegionAt(msg.X, msg.Y) == RegionMessageView`. Recommended **against** — users expect the wheel to work anywhere.

## Files

- `internal/tui/screen.go` — `Region` enum + `RegionAt`.
- `internal/app/app.go` — mouse handler call.
- `internal/tui/screen_test.go` — `RegionAt` tests.

## Tests

```go
func TestRegionAtMessageView(t *testing.T) {
    s := setupScreen(t); s.SetDimensions(80, 20)
    if got := s.RegionAt(10, 0); got != RegionMessageView { t.Errorf(...) }
    if got := s.RegionAt(10, 17); got != RegionMessageView { t.Errorf(...) }
}
func TestRegionAtInput(t *testing.T) {
    s := setupScreen(t); s.SetDimensions(80, 20)
    if got := s.RegionAt(10, 18); got != RegionInput { t.Errorf(...) }
}
func TestRegionAtStatus(t *testing.T) {
    s := setupScreen(t); s.SetDimensions(80, 20)
    if got := s.RegionAt(10, 19); got != RegionStatus { t.Errorf(...) }
}
func TestRegionAtOutOfBounds(t *testing.T) {
    s := setupScreen(t); s.SetDimensions(80, 20)
    if got := s.RegionAt(-1, 5); got != RegionNone { t.Errorf(...) }
    if got := s.RegionAt(10, 99); got != RegionNone { t.Errorf(...) }
    if got := s.RegionAt(80, 5); got != RegionNone { t.Errorf(...) }
}
```

## Risks

- None significant. The change is a strict tightening of click semantics (clicks on input/status are no longer accidentally treated as message clicks).

## Definition of done

- `RegionAt` exists with the four regions.
- `app.go` mouse handler asks `RegionAt`, no longer computes `m.height-2`.
- New region tests pass.
- Manual: scroll up so the `↓N` is visible, click on the status bar — no message should toggle. Click on input — no message should toggle. Click on a tool result row — toggles.

---

# Phase 6 — Keyboard Expand/Collapse

## Goal

Let keyboard-only users expand and collapse individual tool results and thinking blocks, instead of being forced to use a mouse or the all-or-nothing `Ctrl-O`.

## Design

Introduce a "focused message" cursor that selects one toggle-able message at a time, plus three new bindings:

| Key | Action |
|---|---|
| `Tab` | Move focus to the next toggle-able message (forward) |
| `Shift+Tab` | Move focus to the previous toggle-able message |
| `Space` *or* `Enter` (when input is empty) | Toggle the focused message |
| `Esc` (when input is empty and focus is set) | Clear focus |

A message is "toggle-able" if any of these is true:
- `msg.Type == types.MsgToolResult && msg.Summary != ""`
- `msg.ThinkingText != ""`

The focused message is rendered with a visual marker (gutter `▶` or accent border).

`Ctrl-O` is upgraded from "expand all collapsed tool results" to "toggle all": if any toggle-able message is collapsed, expand all; otherwise collapse all.

## Implementation

### 1. Add `focusedIdx` to `MessageView`

```go
// internal/tui/message_view.go

type MessageView struct {
    // ... existing fields ...
    focusedIdx int // -1 = no focus
}

func NewMessageView(theme Theme) *MessageView {
    return &MessageView{theme: theme, focusedIdx: -1}
}

// FocusNextToggleable moves focus to the next toggle-able message.
// Wraps. Returns -1 if there are none.
func (v *MessageView) FocusNextToggleable() int {
    return v.moveFocus(+1)
}

func (v *MessageView) FocusPrevToggleable() int {
    return v.moveFocus(-1)
}

func (v *MessageView) ClearFocus() {
    if v.focusedIdx == -1 { return }
    v.focusedIdx = -1
    v.invalidateCacheAt(v.focusedIdx) // bump fingerprint to redraw without marker
}

// FocusedMessage returns the focused message, or nil.
func (v *MessageView) FocusedMessage() *types.Message {
    if v.focusedIdx < 0 || v.focusedIdx >= len(v.messages) { return nil }
    return v.messages[v.focusedIdx]
}

// EnsureFocusedVisible scrolls so the focused message is in view.
// Pins the scroll (the user is intentionally navigating).
func (v *MessageView) EnsureFocusedVisible() {
    if v.focusedIdx < 0 || v.focusedIdx >= len(v.lineRanges) { return }
    lr := v.lineRanges[v.focusedIdx]
    if lr.startLine < v.scrollPos {
        v.scrollPos = lr.startLine
        v.pinned = true
    } else if lr.endLine > v.scrollPos+v.visible {
        v.scrollPos = max(0, lr.endLine-v.visible)
        v.pinned = true
    }
}

func (v *MessageView) moveFocus(delta int) int {
    n := len(v.messages)
    if n == 0 { v.focusedIdx = -1; return -1 }
    start := v.focusedIdx
    if start < 0 { start = -1 } // so start+1 = 0 on Tab
    for i := 1; i <= n; i++ {
        cand := ((start+i*delta)%n + n) % n
        if v.isToggleable(v.messages[cand]) {
            v.focusedIdx = cand
            return cand
        }
    }
    return -1 // nothing toggle-able
}

func (v *MessageView) isToggleable(msg *types.Message) bool {
    if msg.Type == types.MsgToolResult && msg.Summary != "" { return true }
    if msg.ThinkingText != "" { return true }
    return false
}
```

### 2. Add focus to the fingerprint and renderer

Update `computeFingerprint` to include whether this message is the focused one:

```go
func (v *MessageView) computeFingerprint(msg *types.Message) string {
    focused := false
    if v.focusedIdx >= 0 && v.focusedIdx < len(v.messages) && v.messages[v.focusedIdx] == msg {
        focused = true
    }
    return fmt.Sprintf("%s|%d|%t|%t|%t|%t|%t|%s|%s|%s|%s|%s",
        msg.Type, v.width,
        msg.ThinkingExpanded, msg.Streaming, msg.Collapsed, msg.IsError,
        focused,
        msg.ToolName, msg.ToolArgs, msg.Summary,
        msg.ThinkingText, msg.Content,
    )
}
```

Add a focus marker in `renderMessageLines`. Cleanest approach: prepend a 2-char gutter to every line of the focused message.

```go
func (v *MessageView) renderMessageLines(msg *types.Message) []string {
    // ... existing rendering ...

    // Final pass: if this message is focused, replace the leading "  " of
    // each line with "▶ " styled with theme.Bold.
    focused := v.focusedIdx >= 0 && v.focusedIdx < len(v.messages) && v.messages[v.focusedIdx] == msg
    if focused {
        marker := v.theme.Bold.Render("▶ ")
        for i, line := range lines {
            // Strip a leading 2-space indent if present so widths match.
            if strings.HasPrefix(line, "  ") {
                lines[i] = marker + line[2:]
            } else {
                lines[i] = marker + line
            }
        }
    }
    return lines
}
```

The 2-space leading indent is already used for tool results / thinking lines; verify by reading the existing renderer.

### 3. Wire keys in `app.go`

```go
case tea.KeyTab:
    if m.agentRunning { break }
    m.Screen.GetMessageView().FocusNextToggleable()
    m.Screen.GetMessageView().EnsureFocusedVisible()

case tea.KeyShiftTab:
    if m.agentRunning { break }
    m.Screen.GetMessageView().FocusPrevToggleable()
    m.Screen.GetMessageView().EnsureFocusedVisible()

case tea.KeySpace:
    if m.agentRunning || m.Screen.GetInputArea().Value() != "" {
        // Space goes to input as a normal character.
        // Fall through to default rune handling.
        return m.handleInputRune(msg)
    }
    if focused := m.Screen.GetMessageView().FocusedMessage(); focused != nil {
        m.toggleMessage(focused)
        m.Screen.GetMessageView().MessageGrew()
    }
```

For `KeyEsc`: extend the existing handler to also clear focus when input is empty and no agent is running. Don't disturb the existing `abortAgent` call.

For `KeyEnter` toggle-while-empty-input: only do this if focus is set. Otherwise the current "submit empty input does nothing" behavior is preserved.

```go
case tea.KeyEnter:
    if msg.Alt {
        m.Screen.GetInputArea().HandleRune('\n')
        break
    }
    if m.agentRunning {
        m.abortAgent()
        break
    }
    input := m.Screen.GetInputArea().Value()
    if input == "" {
        if focused := m.Screen.GetMessageView().FocusedMessage(); focused != nil {
            m.toggleMessage(focused)
            m.Screen.GetMessageView().MessageGrew()
            break
        }
    }
    if input != "" {
        if quitCmd := m.submitMessage(input); quitCmd != nil {
            return m, quitCmd
        }
    }
```

### 4. Helper

```go
// toggleMessage flips the appropriate expansion field of a message.
func (m *AppModel) toggleMessage(msg *types.Message) {
    switch {
    case msg.Type == types.MsgToolResult && msg.Summary != "":
        msg.Collapsed = !msg.Collapsed
    case msg.ThinkingText != "":
        msg.ThinkingExpanded = !msg.ThinkingExpanded
    }
}
```

Use it from both the keyboard handler and the existing mouse-click handler in `app.go`. Removes duplicated code.

### 5. Upgrade `Ctrl-O` to "toggle all"

```go
func (m *AppModel) toggleAllToolResults() {
    anyCollapsed := false
    for _, msg := range m.Messages {
        if msg.Type == types.MsgToolResult && msg.Summary != "" && msg.Collapsed {
            anyCollapsed = true
            break
        }
    }
    target := !anyCollapsed // expand all if any collapsed; else collapse all
    changed := false
    for _, msg := range m.Messages {
        if msg.Type == types.MsgToolResult && msg.Summary != "" && msg.Collapsed != target {
            msg.Collapsed = target
            changed = true
        }
    }
    if changed {
        m.Screen.GetMessageView().MessageGrew()
    }
}
```

Wire `tea.KeyCtrlO` to this. Replaces the existing `expandAllToolResults`.

### 6. Focus invalidation on message list shrink

If `SetMessages` reduces the list and `focusedIdx` now points past the end, reset to -1:

```go
func (v *MessageView) SetMessages(messages []*types.Message) {
    v.messages = messages
    if v.focusedIdx >= len(messages) {
        v.focusedIdx = -1
    }
}
```

## Files

- `internal/tui/message_view.go` — focus state, focus methods, render marker, fingerprint update.
- `internal/app/app.go` — Tab/Shift+Tab/Space/Enter handling, `toggleMessage`, `toggleAllToolResults`.
- `internal/tui/message_view_test.go` — focus tests.

## Tests

- `TestFocusNextSkipsNonToggleable`: build messages [user, assistant, toolResult, user, toolResult], `FocusNextToggleable()`, assert focus is the first toolResult; again, second toolResult; again, wraps to first.
- `TestFocusPrevWrapsBackward`: same setup, two `FocusPrevToggleable()` calls.
- `TestEnsureFocusedVisibleScrollsAndPins`: long conversation, focus a message far above the viewport, assert `scrollPos` moved and `pinned == true`.
- `TestFocusFingerprintInvalidatesCache`: render a message, check cached lines; focus it; render again; assert lines differ (have `▶ ` prefix).
- `TestSetMessagesShrinkResetsFocus`: focus index 5, `SetMessages` with 3 messages, assert focus is -1.
- `TestCtrlOToggleAllCollapsesWhenAllExpanded`: 3 expanded tool results, call toggle-all, assert all collapsed.

## Risks

- The focus marker shifts the visible content of focused messages by 2 columns. Make sure `wordwrap` width math (currently `v.width-2`) still works — the marker replaces existing leading whitespace, so net width is unchanged for tool/thinking rows. For other rows it shifts content right by 2; consider whether `v.width-4` should be used for focused-message wrapping. **Simpler alternative:** use a single-column gutter `▶` (1 cell) and pad with one space; net effect on width is small but non-zero.
- `tea.KeySpace` may not be a distinct constant in bubbletea v1.3.10 — `Space` typically arrives as `tea.KeyRunes` with `Runes == [' ']`. Verify with `grep KeySpace key.go` in bubbletea source. If absent, intercept Space in the rune-handling default arm: when input is empty and focus is set, treat space as toggle.

## Definition of done

- All tests pass.
- Manual: with several tool results in scrollback, press `Tab` repeatedly — focus marker moves through them. Press `Space` — focused result expands. `Tab` again — moves to next. `Esc` — focus clears. `Ctrl-O` — toggles all.

---

# Phase 7 — Multiline Input Rendering

## Goal

Render the input area with as many rows as it has logical lines (separated by `\n`), so users can see what they typed via `Alt+Enter`. The message view shrinks accordingly.

## Current behavior (the bug)

`InputArea.Render()` always returns one row. `Alt+Enter` (handled in `app.go` as `m.Screen.GetInputArea().HandleRune('\n')`) inserts a `\n` into `value`, but the renderer never displays it. The user sees only the part before (or after, depending on truncation) the cursor.

## Design

`InputArea` becomes a multi-row renderer. `Screen.SetDimensions` reserves variable space (1 + extra rows for additional lines) for input. `Update()` calls `Screen.SetDimensions` whenever the input's row count changes (or the screen recomputes layout each frame from the current input height — see implementation note).

## Implementation

### 1. `InputArea.LineCount()` and multi-row `Render()`

```go
// internal/tui/input.go

func (i *InputArea) LineCount() int {
    if i.value == "" { return 1 }
    return strings.Count(i.value, "\n") + 1
}

func (i *InputArea) Render() string {
    lines := strings.Split(i.value, "\n")
    if len(lines) == 0 { lines = []string{""} }

    // Locate cursor's logical line and column.
    cursorLine, cursorCol := 0, i.cursorPos
    for li, line := range lines {
        if cursorCol <= utf8.RuneCountInString(line) {
            cursorLine = li
            break
        }
        cursorCol -= utf8.RuneCountInString(line) + 1 // +1 for the \n
    }

    var rows []string
    for li, line := range lines {
        rendered := i.renderSingleLine(line, li == 0, li == cursorLine, cursorCol)
        rows = append(rows, rendered)
    }
    return strings.Join(rows, "\n")
}

// renderSingleLine renders one logical line of the input. Only the first
// line gets the prompt prefix; only the cursor's line gets the cursor.
func (i *InputArea) renderSingleLine(line string, withPrompt bool, withCursor bool, cursorCol int) string {
    // ... refactor of existing Render() body, parameterized on whether to
    // draw the prompt and the cursor.
}
```

Existing single-line `Render()` is refactored to pull the per-line drawing into `renderSingleLine`. Width clamping and cursor placement are unchanged for single-line input — invariant: `LineCount() == 1` produces a byte-identical output to the current renderer.

### 2. `Screen.SetDimensions` reserves variable rows

```go
// internal/tui/screen.go

func (s *Screen) SetDimensions(w, h int) {
    s.width = w
    s.height = h
    s.relayout()
}

func (s *Screen) relayout() {
    inputRows := s.inputArea.LineCount()
    if inputRows < 1 { inputRows = 1 }
    contentHeight := s.height - inputRows - 1 // -1 for status
    if contentHeight < 1 { contentHeight = 1 }

    s.msgView.SetDimensions(s.width, contentHeight)
    s.msgView.SetReservedLines(inputRows + 1)
    s.inputArea.SetWidth(s.width)
    s.statusBar.SetWidth(s.width)
}
```

### 3. Trigger relayout when input grows or shrinks

The cleanest place: `Screen.SetInputValue` and any input-mutating method on `Screen`.

```go
func (s *Screen) SetInputValue(v string) {
    before := s.inputArea.LineCount()
    s.inputArea.SetValue(v)
    if s.inputArea.LineCount() != before {
        s.relayout()
    }
}
```

But `app.go` mutates the input via `m.Screen.GetInputArea().HandleRune(...)` directly, bypassing `Screen`. Two options:

- **A:** Wrap input mutations through `Screen` (`Screen.HandleInputRune(r)`, `Screen.HandleInputKey(k)`). Each call relayouts if the line count changed. Cleanest API but requires a small refactor in `app.go`.
- **B:** Recompute layout at the top of every `Render()`, comparing `inputArea.LineCount()` to the cached `lastInputRows`. Cheaper to wire (no app.go changes), but reintroduces "Render mutates state" — exactly what Phase 2 forbade. **Reject B.**

Go with **A**.

```go
// internal/tui/screen.go
func (s *Screen) HandleInputRune(r rune) {
    before := s.inputArea.LineCount()
    s.inputArea.HandleRune(r)
    if s.inputArea.LineCount() != before { s.relayout() }
}
func (s *Screen) HandleInputKey(k tea.KeyType) (handled bool) {
    before := s.inputArea.LineCount()
    handled = s.inputArea.HandleKey(k)
    if s.inputArea.LineCount() != before { s.relayout() }
    return
}
func (s *Screen) ClearInput() {
    s.inputArea.SetValue("")
    s.inputArea.PushHistory()
    s.relayout()
}
```

In `app.go`, replace direct calls:

```go
m.Screen.GetInputArea().HandleRune(r)         // → m.Screen.HandleInputRune(r)
m.Screen.GetInputArea().HandleKey(msg.Type)   // → m.Screen.HandleInputKey(msg.Type)
m.Screen.GetInputArea().SetValue("")          // → m.Screen.ClearInput()  (in submitMessage path)
```

### 4. Cursor key handling for multiline

Up/Down inside a multi-line input should move the cursor *vertically* between lines, not navigate history. This conflicts with Phase 4's history binding.

Resolution: when input has multiple lines, Up/Down move within the input; when single-line, Up/Down go to history (current Phase-4 behavior). Concretely, `InputArea.HandleKey(tea.KeyUp)`:

- If cursor is on first line: `return false` (let app.go fall through to history).
- Else: move cursor to the same column on the previous line.
- Symmetric for `KeyDown`.

This means `app.go` should call `HandleKey` first and only do history/scroll if the input returned `false`. The current logic does the opposite (decides routing first). Rework:

```go
case tea.KeyUp, tea.KeyDown:
    if !m.agentRunning && m.Screen.GetInputArea().Value() != "" {
        if m.Screen.HandleInputKey(msg.Type) {
            break // consumed by input (e.g. multi-line nav, or history)
        }
    }
    // not consumed: scroll
    if msg.Type == tea.KeyUp {
        m.Screen.GetMessageView().ScrollUp()
    } else {
        m.Screen.GetMessageView().ScrollDown()
    }
```

This requires `InputArea.HandleKey(KeyUp)` to:
- Return `true` if it moved the cursor within multiline input.
- Return `true` if it switched history entry.
- Return `false` only if it can't do either (single-line at top history).

Tweak the existing implementation accordingly.

### 5. RegionAt update from Phase 5

```go
func (s *Screen) RegionAt(x, y int) Region {
    inputTop := s.msgView.height
    inputRows := s.inputArea.LineCount()
    statusTop := inputTop + inputRows
    switch {
    case y < 0 || y >= s.height || x < 0 || x >= s.width:
        return RegionNone
    case y < inputTop:
        return RegionMessageView
    case y < statusTop:
        return RegionInput
    case y < statusTop+1:
        return RegionStatus
    default:
        return RegionNone
    }
}
```

Phase 5 must already be done (the regions naturally handle variable input height).

## Files

- `internal/tui/input.go` — `LineCount`, multi-line `Render`, multi-line cursor handling.
- `internal/tui/screen.go` — `relayout`, `HandleInputRune`, `HandleInputKey`, `ClearInput`, updated `RegionAt`.
- `internal/app/app.go` — replace direct input mutations with screen wrappers; rework Up/Down arm.
- `internal/tui/input_test.go` — multi-line tests.

## Tests

- `TestInputLineCount`: empty → 1; "abc" → 1; "abc\ndef" → 2; "abc\ndef\n" → 3.
- `TestInputRenderShowsAllLines`: SetValue("a\nb\nc"), Render() contains all of "a", "b", "c".
- `TestInputCursorMovesBetweenLines`: SetValue("foo\nbar"), set cursor to position 5 (in "bar"), HandleKey(KeyUp), assert cursor is at position 1 (in "foo" at column 1).
- `TestScreenRelayoutOnMultilineInput`: SetDimensions(80, 20); msgView height = 18. `Screen.HandleInputRune('\n')`; assert `msgView.height == 17`.
- `TestRegionAtMultilineInput`: 3-line input, assert `RegionAt(_, h-4)` → `RegionInput` (3 input rows + 1 status), `RegionAt(_, h-1)` → `RegionStatus`, `RegionAt(_, h-5)` → `RegionMessageView`.

## Risks

- The interaction with the `MessageView`'s Phase-2 `pinned` semantics: when `relayout` shrinks `msgView.visible`, scrollPos may now be past the new `maxScroll`. The defensive clamp in `Render()` handles this. But if the user was at the bottom (not pinned), they may want to follow — call `s.msgView.maybeFollowTail()` from inside `relayout` after the dimension change. **Check that `maybeFollowTail` is package-private vs public** — make it public if needed.
- Markdown renderer is rebuilt on every width change; line-count changes don't trigger that. Fine.
- Cursor blink in multi-line input: if you have a separate cursor frame (you don't currently — input cursor is static `▌`), make sure the cursor only renders on the cursor's line. The implementation above already does this.

## Definition of done

- All input tests pass; old single-line tests still pass byte-for-byte.
- Manual: type `Alt+Enter` to insert a newline, see two input rows. Type more, see content on the second line. Cursor up/down moves between lines without touching history. Submit (Enter) clears input and message view height grows back.

---

# Suggested Order

1. **Phase 4** — biggest UX win, smallest blast radius. Land first.
2. **Phase 5** — small, mechanical, sets up Phase 7. Land second.
3. **Phase 6** — adds visible new behavior (focus marker, Tab nav). Land third.
4. **Phase 7** — most invasive (touches `InputArea`, `Screen`, `app.go`, key routing). Land last so prior phases' tests cover it.

Each phase remains independently shippable and revertable.

# What's Out of Scope (still)

- Search-in-scrollback.
- Copy-to-clipboard from the message view.
- Resizable panes / sidebar.
- Inline image / chart rendering.
- Configurable keybindings.

# Files of Interest (current state)

- `internal/tui/message_view.go` — render cache, scroll math, pinned state.
- `internal/tui/screen.go` — fixed three-row layout, `LinesBelow → SetScrollHint` wiring.
- `internal/tui/statusbar.go` — `↓N` segment.
- `internal/tui/input.go` — single-line input today.
- `internal/tui/theme.go` — styles.
- `internal/app/app.go` — bubbletea `Update`/`View`, key + mouse routing, agent event handlers (all of which now call `MessageGrew()` on content arrival).

# Cleanup After All Phases Land

- Delete `PLAN-improve-TUI.md`, `SCROLLING_BUG_ANALYSIS.md`, `PLAN-scroll-indicator.md`, this file.
- Add a brief `internal/tui/README.md` (or top-of-file comment in `screen.go`) documenting the layout invariants and pinned semantics so future work doesn't reintroduce the autoScroll-style bugs.
