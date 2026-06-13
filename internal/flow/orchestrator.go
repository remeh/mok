package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/user/mok/internal/agent"
)

// FlowOrchestrator executes a FlowDefinition sequentially.
// Each agent runs with its own isolated context; only a handoff
// summary bridges consecutive steps.
type FlowOrchestrator struct {
	agents  map[string]AgentDefinition
	flows   map[string][]string
	factory *AgentFactory
}

// NewFlowOrchestrator creates a new orchestrator.
func NewFlowOrchestrator(
	agents map[string]AgentDefinition,
	flows map[string][]string,
	factory *AgentFactory,
) *FlowOrchestrator {
	return &FlowOrchestrator{
		agents:  agents,
		flows:   flows,
		factory: factory,
	}
}

// Run executes a named flow given the user's original message.
//
// Events from each agent are forwarded to the caller's event channel,
// except for per-agent EventTurnStart/EventTurnEnd which are suppressed.
// The flow emits its own lifecycle events instead: EventFlowStart,
// EventFlowStepStart, EventFlowStepEnd, EventFlowEnd.
//
// Returns the final result and all intermediate results.
func (o *FlowOrchestrator) Run(
	ctx context.Context,
	flowName string,
	userMessage string,
	events chan<- agent.Event,
) (last *AgentRunResult, all []AgentRunResult, err error) {
	steps, ok := o.flows[flowName]
	if !ok {
		return nil, nil, fmt.Errorf("unknown flow: %q", flowName)
	}

	if len(steps) == 0 {
		return nil, nil, fmt.Errorf("flow %q has no steps", flowName)
	}

	// Collect agent names for the flow_start event
	agentNames := make([]string, len(steps))
	copy(agentNames, steps)

	events <- agent.EventFlowStart{
		FlowName: flowName,
		Steps:    agentNames,
	}

	var totalTokens int

	for i, agentName := range steps {
		// Check cancellation before each step
		select {
		case <-ctx.Done():
			events <- agent.EventFlowEnd{
				FlowName:    flowName,
				Completed:   false,
				TotalSteps:  i,
				TotalTokens: totalTokens,
				Error:       ctx.Err(),
			}
			return last, all, fmt.Errorf("flow cancelled after step %d: %w", i, ctx.Err())
		default:
		}

		def := o.agents[agentName]

		// Emit step start
		events <- agent.EventFlowStepStart{
			AgentName:  agentName,
			StepIndex:  i,
			TotalSteps: len(steps),
		}

		// Build the message for this agent
		msg := o.buildStepMessage(i, agentName, def, userMessage, all)

		// Build the agent
		agt, buildErr := o.factory.BuildAgent(def)
		if buildErr != nil {
			result := &AgentRunResult{
				AgentName: agentName,
				Error:     buildErr,
			}
			all = append(all, *result)

			events <- agent.EventFlowStepEnd{
				AgentName: agentName,
				StepIndex: i,
				Error:     buildErr,
			}
			events <- agent.EventFlowEnd{
				FlowName:    flowName,
				Completed:   false,
				TotalSteps:  i + 1,
				TotalTokens: totalTokens,
				Error:       buildErr,
			}
			return last, all, buildErr
		}

		// Run the agent's turn
		startTime := time.Now()
		agentEvents := make(chan agent.Event, 64)

	errCh := make(chan error, 1)

		go func() {
			errCh <- agt.Run(ctx, msg, agentEvents)
			close(agentEvents)
		}()

		// Forward agent events, suppressing per-agent turn boundaries.
		var finalContent string

		for ev := range agentEvents {
			// Suppress per-agent turn boundaries — flow events replace them
			switch ev.(type) {
			case agent.EventTurnStart, agent.EventTurnEnd:
				continue
			}

			events <- ev

			switch e := ev.(type) {
			case agent.EventTextDelta:
				finalContent += e.Text
			case agent.EventMessageEnd:
				if e.Usage != nil {
					totalTokens += e.Usage.TotalTokens
				}
			}
		}


		// Collect agent's messages for summarization
		agentMessages := agt.Messages()

		runErr := <-errCh

		// Build result
		result := &AgentRunResult{
			AgentName:    agentName,
			FinalMessage: finalContent,
			Messages:     agentMessages,
			Error:        runErr,
			StartTime:    startTime,
			EndTime:      time.Now(),
		}

		all = append(all, *result)
		last = result

		// Build summary for the handoff
		summary := BuildHandoffSummary(agentMessages)

		events <- agent.EventFlowStepEnd{
			AgentName: agentName,
			StepIndex: i,
			Summary:   summary,
			Error:     runErr,
		}

		// Release agent for GC
		agt = nil


		// If the agent failed, stop the flow
		if runErr != nil {
			events <- agent.EventFlowEnd{
				FlowName:    flowName,
				Completed:   false,
				TotalSteps:  i + 1,
				TotalTokens: totalTokens,
				Error:       runErr,
			}
			return last, all, fmt.Errorf("agent %q failed: %w", agentName, runErr)
		}
	}

	// Flow completed successfully
	events <- agent.EventFlowEnd{
		FlowName:    flowName,
		Completed:   true,
		TotalSteps:  len(steps),
		TotalTokens: totalTokens,
	}

	return last, all, nil
}

// buildStepMessage constructs the input message for the agent at the given step.
func (o *FlowOrchestrator) buildStepMessage(
	stepIndex int,
	agentName string,
	def AgentDefinition,
	userMessage string,
	previousResults []AgentRunResult,
) string {
	if stepIndex == 0 {
		// First agent: receives the user's message directly
		return userMessage
	}

	// Subsequent agents: receive a handoff from the previous agent
	prev := previousResults[stepIndex-1]
	prevDef := o.agents[prev.AgentName]

	summary := BuildHandoffSummary(prev.Messages)

	return BuildHandoffMessage(HandoffOptions{
		OriginalGoal:      userMessage,
		PreviousAgentName: prev.AgentName,
		PreviousAgentRole: prevDef.Prompt,
		Summary:           summary,
		CurrentAgentName:  agentName,
		CurrentAgentRole:  def.Prompt,
	})
}

// ListFlows returns available flow names.
func (o *FlowOrchestrator) ListFlows() []string {
	names := make([]string, 0, len(o.flows))
	for name := range o.flows {
		names = append(names, name)
	}
	return names
}

// HasFlow returns true if the named flow exists.
func (o *FlowOrchestrator) HasFlow(name string) bool {
	_, ok := o.flows[name]
	return ok
}
