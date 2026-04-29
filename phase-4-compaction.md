# Phase 4: Automatic Compaction

## Goals
- Detect when context is approaching the model's limit
- Smart cut point selection (cut at message/turn boundaries)
- LLM-driven summarization of removed history
- Preserve recent context and file operation history
- Seamless integration with the agent loop

## When to Compact

```go
type CompactionConfig struct {
    MaxContextTokens    int     // Model context window (e.g., 131072)
    Threshold           float64 // Compact at this fraction (e.g., 0.8 = 80%)
    KeepRecentTokens    int     // Always keep this many tokens at the end
}

func ShouldCompact(currentTokens, maxTokens int, threshold float64) bool {
    return currentTokens > int(float64(maxTokens)*threshold)
}
```

Compaction is triggered before each LLM call in the agent loop:
1. Estimate total context tokens
2. If over threshold → compact
3. Re-estimate and retry (recursive protection)

## Smart Cut Points

Cut at message boundaries, never in the middle of a turn:

```
[system] [user1] [assistant1] [tool_result] [user2] [assistant2] [tool_result] ... [user_N] [assistant_N]
          ^cut here^                                    ^preferred^     ^keep recent^
```

Algorithm:
1. Start from the oldest message after system prompt
2. Find the earliest user message that, when everything before it is removed,
   brings total tokens below the target
3. Never cut in the middle of a turn (user + assistant + tool results)
4. Always keep at least `KeepRecentTokens` worth of messages

```go
type CutPoint struct {
    RemoveBeforeIndex int  // Remove all messages before this index
    TokensToRemove    int  // Estimated tokens being removed
    TokensRemaining   int  // Estimated tokens after removal
}

func FindCutPoint(messages []Message, targetTokens, keepRecentTokens int) CutPoint
```

## LLM-Driven Summarization

The removed history is summarized by the LLM into a structured checkpoint:

### Summarization Prompt

```
You are a context summarization assistant. Read the conversation below and
produce a structured summary that allows another LLM to continue the work.

Do NOT continue the conversation. ONLY output the structured summary.

Format:
<summary>
## Goal
[What the user is trying to accomplish]

## Context
[Relevant background information]

## Files Read
[List of files read and key findings]

## Files Modified
[List of files edited/written and what changed]

## Current State
[What has been done, what remains, current thinking]
</summary>
```

### Summarization Flow

```
1. Extract messages to summarize (from cut point to oldest)
2. Serialize conversation to text
3. Send to LLM with summarization prompt
4. Parse summary response
5. Insert summary as a special "compaction_summary" message
6. Keep messages from cut point onward
```

### Summary Message

The summary is inserted as a special message type:

```go
type CompactionSummary struct {
    OriginalTokens  int       // Tokens before compaction
    SummaryTokens   int       // Tokens in the summary
    MessagesRemoved int       // Number of messages summarized
    SummaryText     string    // The actual summary
    Timestamp       time.Time
}
```

In the message history, this appears as:

```
[system]
[compaction_summary: "## Goal\n...\n## Current State\n..."]
[user_N-2]
[assistant_N-2]
...
[user_N]
[assistant_N]
```

## File Operation Tracking

During summarization, extract file operations from the conversation:

```go
type FileOperations struct {
    ReadFiles    []string  // Files that were read
    WrittenFiles []string  // Files that were written
    EditedFiles  []string  // Files that were edited
}

// ExtractFileOps scans assistant messages for tool calls and extracts file paths.
func ExtractFileOps(messages []Message) FileOperations
```

This ensures the summary includes accurate file state information.

## Compaction Orchestrator

```go
package compaction

type Compactor struct {
    client  *llm.Client
    config  CompactionConfig
    model   string  // Can use a smaller/cheaper model for summarization
}

func NewCompactor(client *llm.Client, config CompactionConfig) *Compactor

// Compact reduces the message list to fit within the context window.
// Returns the compacted message list and compaction metadata.
func (c *Compactor) Compact(ctx context.Context, messages []Message) ([]Message, *CompactionResult, error)

type CompactionResult struct {
    TokensBefore  int
    TokensAfter   int
    MessagesRemoved int
    Summary       string
}
```

### Integration with Agent Loop

```go
// In agent.Run():
func (a *Agent) buildContext() []Message {
    messages := a.buildMessageList()

    tokens := a.tracker.EstimateTokens(messages)
    if compaction.ShouldCompact(tokens, a.config.MaxContextTokens, a.config.CompactionThreshold) {
        compacted, result, err := a.compactor.Compact(ctx, messages)
        if err != nil {
            // Fallback: hard cut at message boundaries
            compacted = a.hardCut(messages)
        }
        // Update history
        a.messages = compacted
        tokens = result.TokensAfter
    }

    return messages
}
```

## Fallback: Hard Cut

If LLM summarization fails (network error, model unavailable):

```go
func (a *Agent) hardCut(messages []Message) []Message {
    // Remove oldest messages (after system prompt) until under threshold
    // Cut at turn boundaries only
    // Insert a "[previous context removed]" marker
}
```

## Tasks

1. [ ] Implement `internal/compaction/context.go`: Token estimation, shouldCompact
2. [ ] Implement `internal/compaction/cutpoint.go`: Smart cut point selection
3. [ ] Implement `internal/compaction/file_ops.go`: File operation extraction
4. [ ] Implement `internal/compaction/prompts.go`: Summarization prompts
5. [ ] Implement `internal/compaction/summarizer.go`: LLM-driven summarization
6. [ ] Implement `internal/compaction/compaction.go`: Compaction orchestrator
7. [ ] Integrate compaction into agent loop
8. [ ] Add compaction events to the event stream
9. [ ] Test: Long conversation triggers compaction, summary is accurate