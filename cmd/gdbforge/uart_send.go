package main

import (
	"fmt"
	"io"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/yairgd/gdbforge/internal/devport"
	"github.com/yairgd/gdbforge/internal/luahost"
)

func writePayload(w io.WriteCloser, device string, payload []byte) error {
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("uart_send %s: %w", device, err)
	}
	return nil
}

func sendUartLine(device string, baud int, line string) error {
	device = strings.TrimSpace(device)
	line = strings.TrimSpace(line)
	if device == "" {
		return fmt.Errorf("uart_send: empty device")
	}
	if line == "" {
		return fmt.Errorf("uart_send: empty line")
	}
	payload := []byte(line + "\r\n")

	port, err := devport.Open(device, baud)
	if err != nil {
		return err
	}
	defer port.Close()
	return writePayload(port, device, payload)
}

func (c *luaCtl) installUartAPI(rt *luahost.Runtime) {
	if rt == nil {
		return
	}
	rt.SetGdbforgeFunc("uart_send", func(L *lua.LState) int {
		device := strings.TrimSpace(L.CheckString(1))
		baud := 115200
		line := ""
		switch L.GetTop() {
		case 2:
			line = L.CheckString(2)
		case 3:
			baud = int(L.CheckNumber(2))
			line = L.CheckString(3)
		default:
			L.RaiseError("uart_send(device, baud, line) or uart_send(device, line)")
			return 0
		}
		if err := sendUartLine(device, baud, line); err != nil {
			L.RaiseError("%s", err.Error())
			return 0
		}
		return 0
	})
}
