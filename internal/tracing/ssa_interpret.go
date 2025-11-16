package tracing

import (
	"golang.org/x/tools/go/ssa"
)

// InterpretSSA interpret traversed SSA graph paths using explicit DFS stack.
// Each path is explored once with isolated state copy.
func InterpretSSA(fn *ssa.Function, ctx *Context) {
	if fn == nil || len(fn.Blocks) == 0 {
		return
	}

	type frame struct {
		block *ssa.BasicBlock
		state *State
	}

	stack := []frame{{fn.Blocks[0], NewState()}}
	visited := make(map[*ssa.BasicBlock]bool)

	for len(stack) > 0 {
		// Pop frame
		n := len(stack) - 1
		f := stack[n]
		stack = stack[:n]

		if visited[f.block] {
			continue
		}
		visited[f.block] = true

		newState := f.state.Clone()
		traceBlock(f.block, ctx, newState)

		// Push successors
		for _, succ := range f.block.Succs {
			stack = append(stack, frame{succ, newState.Clone()})
		}
	}
}

// traceBlock performs branch-level interpretation of SSA instructions.
// It updates the State according to detected operations on errors
// and records transitions in tracing logs when appropriate.
func traceBlock(block *ssa.BasicBlock, ctx *Context, state *State) {
	for _, instr := range block.Instrs {
		interpret(instr, ctx, state)
	}
}

// interpret decodes SSA instructions and applies semantic updates.
// The function recognizes patterns related to error handling:
//   - assignments from call results,
//   - checks against nil,
//   - calls to loggers or wrappers,
//   - exits and propagations.
func interpret(instr ssa.Instruction, ctx *Context, state *State) {
	switch v := instr.(type) {

	// Example: "t0 = call f()"
	case *ssa.Call:
		handleCall(v, ctx, state)

	// Example: "if t1 != nil"
	case *ssa.If:
		handleIf(v, ctx, state)

	// Example: "return err"
	case *ssa.Return:
		handleReturn(v, ctx, state)

	// Assignment or other generic instruction.
	default:
		handleAssign(v, ctx, state)
	}
}

// --- handlers ---

func handleCall(call *ssa.Call, ctx *Context, state *State) {
	// TODO: recognize error-producing calls, wrappers, and loggers.
}

func handleIf(cond *ssa.If, ctx *Context, state *State) {
	// TODO: detect conditional checks like "if err != nil".
}

func handleReturn(ret *ssa.Return, ctx *Context, state *State) {
	// TODO: mark error as propagated or logged.
}

func handleAssign(instr ssa.Instruction, ctx *Context, state *State) {
	// TODO: track variable assignments involving errors.
}
