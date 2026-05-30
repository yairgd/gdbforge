# Project Architecture

This document describes the main architecture of the project in terms of its types and their relationships.

## Mermaid Diagram: Core Events Layer
```mermaid
classDiagram
    direction TB

    class Event {
        <<interface>>
        +Type() string
    }

    class Emitter {
        <<Function Type>>
        +Emit(event: Event)
    }

    class SubmitMessage {
        +Text string
        +Type() string
    }

    class RunCommand {
        +Command string
        +Type() string
    }

    class MessageSent {
        +Text string
        +Type() string
    }

    class Quit {
        +Text string
        +Type() string
    }

    class GdbOutputMsg {
        +Data string
        +Err error
        +Type() string
    }

    Event <|.. SubmitMessage
    Event <|.. RunCommand
    Event <|.. MessageSent
    Event <|.. Quit
    Event <|-- GdbOutputMsg
```

## Mermaid Diagram: Terminal UI Node
```mermaid
classDiagram
    direction TB

    class Node {
        +Type NodeType
        +Widget Widget
        +Rect Rect
        +First *Node
        +Second *Node
        +Dir SplitDir
        +Ratio float64
    }

    class NodeType {
        <<Enum>>
        +NodeLeaf
        +NodeSplit
    }

    class SplitDir {
        <<Enum>>
        +Horizontal
        +Vertical
    }

    Node *-- Node
    Node --> NodeType
    Node --> SplitDir
```

## Notes
- The `Event` interface in the Core module is extended by several struct types, each of which represents a specific kind of event.
- The `Node` struct in the Terminal UI module is a composite structure supporting both "Leaf" (simple widgets) and "Split" (divided layouts) nodes, providing flexibility for UI layout designs.