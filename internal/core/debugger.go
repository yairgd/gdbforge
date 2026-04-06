package core

type Debugger interface {
	Send(cmd string) error
	SendRaw(raw string) error
}
