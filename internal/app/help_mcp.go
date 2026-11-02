package app

import (
	"fmt"
	"io"
)

// WantsHelpMCP reports `gdbforge help -mcp` (also `help mcp`, `-h -mcp`, `--help --mcp`).
// Like -version it is answered before the TUI or the flag parser starts.
func WantsHelpMCP(args []string) bool {
	if len(args) != 2 {
		return false
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
	default:
		return false
	}
	switch args[1] {
	case "mcp", "-mcp", "--mcp":
		return true
	}
	return false
}

// HelpMCPAgentTools are the tools the agent prompt below names; a test keeps them in step
// with the service registry.
var HelpMCPAgentTools = []string{
	"interrupt", "continue", "wait_for_stop", "step", "next", "stepi", "finish", "run_to",
	"read_memory", "registers", "evaluate", "list_breakpoints", "set_breakpoint", "clear_breakpoint",
	"list_frames", "list_threads", "batch", "gdb_command",
}

// HelpMCP prints how to let an AI agent drive gdbforge, and the prompt to give the agent.
func HelpMCP(w io.Writer) {
	fmt.Fprint(w, helpMCPText)
}

const helpMCPText = `gdbforge for AI agents (REST and MCP)

1. Start gdbforge with a token of your choice (loopback addresses only):

     gdbforge -serve 127.0.0.1:7777 -serve-token <token> <app>

   <app> is your program or ELF, or nothing to attach later (gdbforge -g gdb ...).
   A token: head -c16 /dev/urandom | xxd -p
   To keep it out of ps: GDBFORGE_TOKEN=<token> gdbforge -serve 127.0.0.1:7777 <app>

2. Connect the agent.

   REST   curl -s -H "Authorization: Bearer <token>" http://127.0.0.1:7777/api            # tools + arguments
          curl -s -H "Authorization: Bearer <token>" http://127.0.0.1:7777/api/interrupt -d '{}'
   MCP    claude mcp add --transport http gdbforge http://127.0.0.1:7777/mcp \
              --header "Authorization: Bearer <token>"

3. Give the agent this prompt (replace <token>; for MCP the connection already has it):

--------------------------------------------------------------------------------
You can debug the program through gdbforge, a GDB front-end, over HTTP.

Endpoint: http://127.0.0.1:7777   Header: Authorization: Bearer <token>
Call a tool with POST /api/<tool> and a JSON body. GET /api lists every tool with its arguments.

Every reply says where the target is: "state" is running, stopped or disconnected, and when
stopped there are also stop_reason, pc, func, file, line and, if a breakpoint or watchpoint
caused the stop, bkpt_number. Trust it; do not sleep, poll, or read memory just to find out
whether the target is halted. Errors carry a code: target_running (call interrupt first),
interrupt_timeout, timeout, not_supported.

Run control (each waits and returns the new state):
  interrupt                 halt a running target
  continue {wait_ms}        resume; returns state running if it did not stop within wait_ms
  wait_for_stop {timeout_ms} keep waiting for a running target to stop
  step, next, stepi, finish
  run_to {location, timeout_ms}   temporary breakpoint + continue + wait, in one call
Inspection (the target must be stopped):
  read_memory {address, length}   hex, plus words32 when length is a multiple of 4
  registers {names}, evaluate {expression}, list_frames, list_threads
  batch {ops:[{tool,args},...], halt, resume}
        several reads on one halted snapshot in one call; with halt+resume a running
        target is halted, read, and resumed (not resumed if a breakpoint stopped it)
Breakpoints: list_breakpoints (number, location, hits, temporary), set_breakpoint, clear_breakpoint
gdb_command {command, format}  any GDB command; the reply is clean console text
                               (format "mi" for the raw MI records)

Notes:
- Start with list_breakpoints. The user may have breakpoints in the code you test; a stop with
  bkpt_number set is that breakpoint, not a hang.
- Use batch for a consistent snapshot of several registers or memory ranges.
- The state is GDB's own view. monitor halt, monitor go and monitor reset change the target
  without GDB noticing, so the state can be wrong after them; prefer the run-control tools.
- Do not write memory, registers or breakpoints unless asked, and say what you changed.
--------------------------------------------------------------------------------

More: README "Driving gdbforge from an AI agent", docs/LUA_API.md "Services over REST and MCP".
`
