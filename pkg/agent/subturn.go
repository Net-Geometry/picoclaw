package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/tools"
)

// ====================== Config & Constants ======================
const (
	maxSubTurnDepth       = 3
	maxConcurrentSubTurns = 5
	// concurrencyTimeout is the maximum time to wait for a concurrency slot.
	// This prevents indefinite blocking when all slots are occupied by slow sub-turns.
	concurrencyTimeout = 30 * time.Second
	// maxEphemeralHistorySize limits the number of messages stored in ephemeral sessions.
	// This prevents memory accumulation in long-running sub-turns.
	maxEphemeralHistorySize = 50
	// defaultSubTurnTimeout is the default maximum duration for a SubTurn.
	// SubTurns that run longer than this will be cancelled.
	defaultSubTurnTimeout = 5 * time.Minute
)

var (
	ErrDepthLimitExceeded   = errors.New("sub-turn depth limit exceeded")
	ErrInvalidSubTurnConfig = errors.New("invalid sub-turn config")
	ErrConcurrencyTimeout   = errors.New("timeout waiting for concurrency slot")
)

// ====================== SubTurn Config ======================

// SubTurnConfig configures the execution of a child sub-turn.
//
// Usage Examples:
//
// Synchronous sub-turn (Async=false):
//
//	cfg := SubTurnConfig{
//	    Model: "gpt-4o-mini",
//	    SystemPrompt: "Analyze this code",
//	    Async: false,  // Result returned immediately
//	}
//	result, err := SpawnSubTurn(ctx, cfg)
//	// Use result directly here
//	processResult(result)
//
// Asynchronous sub-turn (Async=true):
//
//	cfg := SubTurnConfig{
//	    Model: "gpt-4o-mini",
//	    SystemPrompt: "Background analysis",
//	    Async: true,  // Result delivered to channel
//	}
//	result, err := SpawnSubTurn(ctx, cfg)
//	// Result also available in parent's pendingResults channel
//	// Parent turn will poll and process it in a later iteration
type SubTurnConfig struct {
	Model        string
	Tools        []tools.Tool
	SystemPrompt string
	MaxTokens    int

	// Async controls the result delivery mechanism:
	//
	// When Async = false (synchronous sub-turn):
	//   - The caller blocks until the sub-turn completes
	//   - The result is ONLY returned via the function return value
	//   - The result is NOT delivered to the parent's pendingResults channel
	//   - This prevents double delivery: caller gets result immediately, no need for channel
	//   - Use case: When the caller needs the result immediately to continue execution
	//   - Example: A tool that needs to process the sub-turn result before returning
	//
	// When Async = true (asynchronous sub-turn):
	//   - The sub-turn runs in the background (still blocks the caller, but semantically async)
	//   - The result is delivered to the parent's pendingResults channel
	//   - The result is ALSO returned via the function return value (for consistency)
	//   - The parent turn can poll pendingResults in later iterations to process results
	//   - Use case: Fire-and-forget operations, or when results are processed in batches
	//   - Example: Spawning multiple sub-turns in parallel and collecting results later
	//
	// IMPORTANT: The Async flag does NOT make the call non-blocking. It only controls
	// whether the result is delivered via the channel. For true non-blocking execution,
	// the caller must spawn the sub-turn in a separate goroutine.
	Async bool

	// Critical indicates this SubTurn's result is important and should continue
	// running even after the parent turn finishes gracefully.
	//
	// When parent finishes gracefully (Finish(false)):
	//   - Critical=true: SubTurn continues running, delivers result as orphan
	//   - Critical=false: SubTurn exits gracefully without error
	//
	// When parent finishes with hard abort (Finish(true)):
	//   - All SubTurns are cancelled regardless of Critical flag
	Critical bool

	// Timeout is the maximum duration for this SubTurn.
	// If the SubTurn runs longer than this, it will be cancelled.
	// Default is 5 minutes (defaultSubTurnTimeout) if not specified.
	Timeout time.Duration

	// Can be extended with temperature, topP, etc.
}

// ====================== Sub-turn Events (Aligned with EventBus) ======================
type SubTurnSpawnEvent struct {
	ParentID string
	ChildID  string
	Config   SubTurnConfig
}

type SubTurnEndEvent struct {
	ChildID string
	Result  *tools.ToolResult
	Err     error
}

type SubTurnResultDeliveredEvent struct {
	ParentID string
	ChildID  string
	Result   *tools.ToolResult
}

type SubTurnOrphanResultEvent struct {
	ParentID string
	ChildID  string
	Result   *tools.ToolResult
}

// ====================== Context Keys ======================
type agentLoopKeyType struct{}

var agentLoopKey = agentLoopKeyType{}

// WithAgentLoop injects AgentLoop into context for tool access
func WithAgentLoop(ctx context.Context, al *AgentLoop) context.Context {
	return context.WithValue(ctx, agentLoopKey, al)
}

// AgentLoopFromContext retrieves AgentLoop from context
func AgentLoopFromContext(ctx context.Context) *AgentLoop {
	al, _ := ctx.Value(agentLoopKey).(*AgentLoop)
	return al
}

// ====================== Helper Functions ======================

func (al *AgentLoop) generateSubTurnID() string {
	return fmt.Sprintf("subturn-%d", al.subTurnCounter.Add(1))
}

// ====================== Core Function: spawnSubTurn ======================

// AgentLoopSpawner implements tools.SubTurnSpawner interface.
// This allows tools to spawn sub-turns without circular dependency.
type AgentLoopSpawner struct {
	al *AgentLoop
}

// SpawnSubTurn implements tools.SubTurnSpawner interface.
func (s *AgentLoopSpawner) SpawnSubTurn(ctx context.Context, cfg tools.SubTurnConfig) (*tools.ToolResult, error) {
	parentTS := turnStateFromContext(ctx)
	if parentTS == nil {
		return nil, errors.New("parent turnState not found in context - cannot spawn sub-turn outside of a turn")
	}

	// Convert tools.SubTurnConfig to agent.SubTurnConfig
	agentCfg := SubTurnConfig{
		Model:        cfg.Model,
		Tools:        cfg.Tools,
		SystemPrompt: cfg.SystemPrompt,
		MaxTokens:    cfg.MaxTokens,
		Async:        cfg.Async,
	}

	return spawnSubTurn(ctx, s.al, parentTS, agentCfg)
}

// NewSubTurnSpawner creates a SubTurnSpawner for the given AgentLoop.
func NewSubTurnSpawner(al *AgentLoop) *AgentLoopSpawner {
	return &AgentLoopSpawner{al: al}
}

// SpawnSubTurn is the exported entry point for tools to spawn sub-turns.
// It retrieves AgentLoop and parent turnState from context and delegates to spawnSubTurn.
func SpawnSubTurn(ctx context.Context, cfg SubTurnConfig) (*tools.ToolResult, error) {
	al := AgentLoopFromContext(ctx)
	if al == nil {
		return nil, errors.New("AgentLoop not found in context - ensure context is properly initialized")
	}

	parentTS := turnStateFromContext(ctx)
	if parentTS == nil {
		return nil, errors.New("parent turnState not found in context - cannot spawn sub-turn outside of a turn")
	}

	return spawnSubTurn(ctx, al, parentTS, cfg)
}

func spawnSubTurn(ctx context.Context, al *AgentLoop, parentTS *turnState, cfg SubTurnConfig) (result *tools.ToolResult, err error) {
	// 0. Acquire concurrency semaphore FIRST to ensure it's released even if early validation fails.
	// Blocks if parent already has maxConcurrentSubTurns running, with a timeout to prevent indefinite blocking.
	// Also respects context cancellation so we don't block forever if parent is aborted.
	var semAcquired bool
	if parentTS.concurrencySem != nil {
		// Create a timeout context for semaphore acquisition
		timeoutCtx, cancel := context.WithTimeout(ctx, concurrencyTimeout)
		defer cancel()

		select {
		case parentTS.concurrencySem <- struct{}{}:
			semAcquired = true
			defer func() {
				if semAcquired {
					<-parentTS.concurrencySem
				}
			}()
		case <-timeoutCtx.Done():
			// Check parent context first - if it was cancelled, propagate that error
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// Otherwise it's our timeout
			return nil, fmt.Errorf("%w: all %d slots occupied for %v",
				ErrConcurrencyTimeout, maxConcurrentSubTurns, concurrencyTimeout)
		}
	}

	// 1. Depth limit check
	if parentTS.depth >= maxSubTurnDepth {
		logger.WarnCF("subturn", "Depth limit exceeded", map[string]any{
			"parent_id": parentTS.turnID,
			"depth":     parentTS.depth,
			"max_depth": maxSubTurnDepth,
		})
		return nil, ErrDepthLimitExceeded
	}

	// 2. Config validation
	if cfg.Model == "" {
		return nil, ErrInvalidSubTurnConfig
	}

	// 3. Determine timeout for child SubTurn
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultSubTurnTimeout
	}

	// 4. Create INDEPENDENT child context (not derived from parent ctx).
	// This allows the child to continue running after parent finishes gracefully.
	// The child has its own timeout for self-protection.
	childCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	childID := al.generateSubTurnID()
	childTS := newTurnState(childCtx, childID, parentTS)
	// Set the cancel function so Finish(true) can trigger hard cancellation
	childTS.cancelFunc = cancel

	// IMPORTANT: Put childTS into childCtx so that code inside runTurn can retrieve it
	childCtx = withTurnState(childCtx, childTS)
	childCtx = WithAgentLoop(childCtx, al) // Propagate AgentLoop to child turn

	// 5. Establish parent-child relationship (thread-safe)
	parentTS.mu.Lock()
	parentTS.childTurnIDs = append(parentTS.childTurnIDs, childID)
	parentTS.mu.Unlock()

	// 6. Emit Spawn event
	MockEventBus.Emit(SubTurnSpawnEvent{
		ParentID: parentTS.turnID,
		ChildID:  childID,
		Config:   cfg,
	})

	// 7. Defer cleanup: deliver result (for async), emit End event, and recover from panics
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("subturn panicked: %v", r)
			logger.ErrorCF("subturn", "SubTurn panicked", map[string]any{
				"child_id":  childID,
				"parent_id": parentTS.turnID,
				"panic":     r,
			})
		}

		// Result Delivery Strategy (Async vs Sync)
		if cfg.Async {
			deliverSubTurnResult(parentTS, childID, result)
		}

		MockEventBus.Emit(SubTurnEndEvent{
			ChildID: childID,
			Result:  result,
			Err:     err,
		})
	}()

	// 8. Execute sub-turn via the real agent loop.
	result, err = runTurn(childCtx, al, childTS, cfg)

	return result, err
}

// ====================== Result Delivery ======================

// deliverSubTurnResult delivers a sub-turn result to the parent turn's pendingResults channel.
//
// IMPORTANT: This function is ONLY called for asynchronous sub-turns (Async=true).
// For synchronous sub-turns (Async=false), results are returned directly via the function
// return value to avoid double delivery.
//
// Delivery behavior:
//   - If parent turn is still running: attempts to deliver to pendingResults channel
//   - If channel is full: emits SubTurnOrphanResultEvent (result is lost from channel but tracked)
//   - If parent turn has finished: emits SubTurnOrphanResultEvent (late arrival)
//
// Thread safety:
//   - Reads parent state under lock, then releases lock before channel send
//   - Small race window exists but is acceptable (worst case: result becomes orphan)
//
// Event emissions:
//   - SubTurnResultDeliveredEvent: successful delivery to channel
//   - SubTurnOrphanResultEvent: delivery failed (parent finished or channel full)
func deliverSubTurnResult(parentTS *turnState, childID string, result *tools.ToolResult) {
	// Check parent state under lock, but don't hold lock while sending to channel
	parentTS.mu.Lock()
	isFinished := parentTS.isFinished
	resultChan := parentTS.pendingResults
	parentTS.mu.Unlock()

	// If parent turn has already finished, treat this as an orphan result
	if isFinished || resultChan == nil {
		if result != nil {
			MockEventBus.Emit(SubTurnOrphanResultEvent{
				ParentID: parentTS.turnID,
				ChildID:  childID,
				Result:   result,
			})
		}
		return
	}

	// Parent Turn is still running → attempt to deliver result
	// Note: There's still a small race window between the isFinished check above and the send below,
	// but this is acceptable - worst case the result becomes an orphan, which is handled gracefully.
	select {
	case resultChan <- result:
		// Successfully delivered
		MockEventBus.Emit(SubTurnResultDeliveredEvent{
			ParentID: parentTS.turnID,
			ChildID:  childID,
			Result:   result,
		})
	default:
		// Channel is full - treat as orphan result
		logger.WarnCF("subturn", "pendingResults channel full", map[string]any{
			"parent_id": parentTS.turnID,
			"child_id":  childID,
		})
		if result != nil {
			MockEventBus.Emit(SubTurnOrphanResultEvent{
				ParentID: parentTS.turnID,
				ChildID:  childID,
				Result:   result,
			})
		}
	}
}

// runTurn builds a temporary AgentInstance from SubTurnConfig and delegates to
// the real agent loop. The child's ephemeral session is used for history so it
// never pollutes the parent session.
func runTurn(ctx context.Context, al *AgentLoop, ts *turnState, cfg SubTurnConfig) (*tools.ToolResult, error) {
	// Derive candidates from the requested model using the parent loop's provider.
	defaultProvider := al.GetConfig().Agents.Defaults.Provider
	candidates := providers.ResolveCandidates(
		providers.ModelConfig{Primary: cfg.Model},
		defaultProvider,
	)

	// Build a minimal AgentInstance for this sub-turn.
	// It reuses the parent loop's provider and config, but gets its own
	// ephemeral session store and tool registry.
	parentAgent := al.GetRegistry().GetDefaultAgent()

	// Determine which tools to use: explicit config or inherit from parent
	toolRegistry := tools.NewToolRegistry()
	toolsToRegister := cfg.Tools
	if len(toolsToRegister) == 0 {
		toolsToRegister = parentAgent.Tools.GetAll()
	}
	for _, t := range toolsToRegister {
		toolRegistry.Register(t)
	}

	childAgent := &AgentInstance{
		ID:                        ts.turnID,
		Model:                     cfg.Model,
		MaxIterations:             parentAgent.MaxIterations,
		MaxTokens:                 cfg.MaxTokens,
		Temperature:               parentAgent.Temperature,
		ThinkingLevel:             parentAgent.ThinkingLevel,
		ContextWindow:             parentAgent.ContextWindow, // Inherit from parent agent
		SummarizeMessageThreshold: parentAgent.SummarizeMessageThreshold,
		SummarizeTokenPercent:     parentAgent.SummarizeTokenPercent,
		Provider:                  parentAgent.Provider,
		Sessions:                  ts.session,
		ContextBuilder:            parentAgent.ContextBuilder,
		Tools:                     toolRegistry,
		Candidates:                candidates,
	}
	if childAgent.MaxTokens == 0 {
		childAgent.MaxTokens = parentAgent.MaxTokens
	}

	finalContent, err := al.runAgentLoop(ctx, childAgent, processOptions{
		SessionKey:      ts.turnID,
		UserMessage:     cfg.SystemPrompt,
		DefaultResponse: "",
		EnableSummary:   false,
		SendResponse:    false,
	})
	if err != nil {
		return nil, err
	}
	return &tools.ToolResult{ForLLM: finalContent}, nil
}

// ====================== Other Types ======================
