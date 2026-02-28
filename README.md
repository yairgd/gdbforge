# Chatbot project

PromptCore is a lightweight AI context engine designed to manage long-lived conversational state.

It is built with a clean architecture that separates:

- Core logic (context + AI orchestration)
- Application layer (events + state)
- UI adapters (TUI, future Web UI, etc.)

The goal is to provide a reusable AI context infrastructure that can power:

- Terminal interfaces (TUI)
- Web interfaces
- API servers
- Debugging assistants
- Developer tools

---

## Vision

PromptCore is not just a chat application.

It is a context engine.

It manages:
- Conversation history
- AI orchestration
- Command handling
- Future RAG integration
- Pluggable UI adapters

---



# Project Structure

```bash
promptcore/
├── go.mod
├── go.sum
│
├── cmd/
│   └── tui/
│       └── main.go          # Entry point → tea.NewProgram(...)
│
├── internal/
│
│   ├── core/                # 🧠 Business logic (no UI dependencies)
│   │   ├── session.go       # Session management
│   │   ├── message.go       # Message model
│   │   ├── context.go       # Context manager
│   │   ├── ai.go            # AI integration
│   │   └── commands.go      # Command registry
│   │
│   ├── app/                 # Orchestration layer
│   │   ├── app.go           # Connects UI ↔ core
│   │   ├── state.go         # AppState
│   │   └── events.go        # SubmitMessage, etc.
│   │
│   └── tui/                 # UI adapter layer
│       ├── model.go         # Main tea.Model implementation
│       ├── input.go
│       ├── chat.go
│       ├── command.go
│       └── layout.go
│
├── playground/              # 🧪 Experiments and prototypes
│   ├── bubbletea-ex1/
│   │   └── main.go
│   └── streaming-test/
│       └── main.go
│
└── tests/
    └── core_test.go

```


# Debugging with Delve (dlv)

This project can be debugged using Delve in headless mode.

## 1. Start Delve in Headless Mode

Open a terminal in the project root directory and run:

dlv debug ./cmd/tui --headless --listen=:2346 --api-version=2

This will:
- Compile the project
- Start the debugger
- Listen for remote connections on port 2346
- Wait for a client to connect

## 2. Connect to the Debugger

Open a second terminal and run:

dlv connect :2346

Now you are connected to the running debug session.

## 3. Useful Delve Commands

Inside the Delve prompt you can use:

b main.main        # set breakpoint
c                  # continue execution
n                  # next line
s                  # step into
bt                 # backtrace
p variableName     # print variable
q                  # quit debugger

## 4. Notes

- Make sure the port number matches in both commands.
- Always run the headless debugger from the project root (where go.mod is located).
- If you want to debug another command under cmd/, change the path accordingly.


