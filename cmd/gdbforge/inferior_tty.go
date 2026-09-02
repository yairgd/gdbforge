package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/yairgd/gdbforge/internal/dlv"
	"github.com/yairgd/gdbforge/internal/gdbforge/backend"
)

// SetInferiorTTY switches program stdio to path ("internal" or "" restores the
// in-app IO pane; "external" or a /dev/pts/N path routes stdio to an external
// terminal). GDB switches live via -inferior-tty-set. Delve external stdio uses
// :lua dlv_ext_port; only internal is supported here for -g dlv.
func (a *DebuggerApp) SetInferiorTTY(path string) error {
	path = strings.TrimSpace(path)
	if a.backend == nil {
		return fmt.Errorf("no debugger session")
	}
	if path == "" || path == "internal" {
		a.closeExternalInferiorHold()
	}
	if a.backend.RequiresInferiorTTYRestart() {
		if path == "external" || (path != "" && path != "internal" && strings.HasPrefix(path, "/dev/")) {
			return fmt.Errorf("Delve: use :lua dlv_ext_port for external stdio (not :set inferior-tty)")
		}
		return a.setDlvInferiorTTY(path)
	}
	if path == "external" {
		pts, err := a.OpenExternalTTY()
		if err != nil {
			return err
		}
		path = pts
	}
	if err := a.backend.SetInferiorTTYPath(path); err != nil {
		return err
	}
	a.syncInferiorIOView()
	return nil
}

// setDlvInferiorTTY restarts Delve with --tty path (or an internal PTY).
// path "" / "internal" → in-app IO pane; otherwise an external /dev/pts/N.
func (a *DebuggerApp) setDlvInferiorTTY(path string) error {
	db := a.dlvBackend()
	if db == nil || db.Client == nil {
		return fmt.Errorf("no dlv session")
	}
	ext := ""
	if path != "" && path != "internal" {
		ext = path
	}
	cur := ""
	if db.Client.UsesExternalInferiorTTY() {
		cur = db.Client.InferiorTTYPath()
	}
	if ext == cur {
		// Already on the desired mode/path — still refresh IO chrome.
		if ext != "" {
			a.inferiorIO.markExternal(ext)
		} else if inf := db.Client.InferiorTTY(); inf != nil {
			a.inferiorIO.rewireInternal(inf)
		}
		return nil
	}
	return a.restartDlvWithInferiorTTY(ext)
}

// inferiorTTYSetMsg runs SetInferiorTTY on the next UI turn so :set inferior-tty
// can return immediately and the cmdline clears before opening a terminal.
type inferiorTTYSetMsg struct {
	path string
}

// restartDlvWithInferiorTTY tears down the current Delve PTY session and
// starts a new `dlv exec --tty …` with the same program args.
func (a *DebuggerApp) restartDlvWithInferiorTTY(extTTY string) error {
	a.tearDownDlvSession()

	client, err := dlv.NewClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, dlv.ClientOptions{InferiorTTY: extTTY})
	if err != nil {
		// Never leave the app without a Delve session — Enter would no-op.
		fallback, ferr := dlv.NewClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, dlv.ClientOptions{})
		if ferr != nil {
			return fmt.Errorf("restart dlv with tty %q: %w (also failed to restore internal: %v)", extTTY, err, ferr)
		}
		a.attachRestartedDlv(fallback)
		a.inferiorIO.rewireInternal(fallback.InferiorTTY())
		return fmt.Errorf("external tty failed (%v); restored internal IO", err)
	}
	a.attachRestartedDlv(client)

	if client.UsesExternalInferiorTTY() {
		a.inferiorIO.markExternal(client.InferiorTTYPath())
		if a.ctx.Log != nil {
			a.ctx.Log.Named("dlv").Info("inferior stdio → external tty " + client.InferiorTTYPath() + " (dlv restarted)")
		}
		return nil
	}
	if inf := client.InferiorTTY(); inf != nil {
		a.inferiorIO.rewireInternal(inf)
	}
	if a.ctx.Log != nil {
		a.ctx.Log.Named("dlv").Info("inferior stdio → internal IO pane (dlv restarted)")
	}
	return nil
}

// attachRestartedDlv wires a freshly spawned Delve client into the UI.
func (a *DebuggerApp) attachRestartedDlv(client *dlv.Client) {
	if db := a.dlvBackend(); db != nil {
		db.ReplaceClient(client)
	} else {
		a.backend = backend.NewDLV(client)
	}

	if boot := client.TakeStartupOutput(); boot != "" && a.gdbWidget != nil {
		a.gdbWidget.WriteBoot(boot)
	}
	if cli := client.TTY; cli != nil && a.gdbWidget != nil {
		a.console.wireCLI(a.gdbWidget, cli, a.RequestFrame)
	}
	a.console.startGdbConsoleBridge()

	if a.gdbMcp != nil {
		a.gdbMcp.SetSession(a.GDB())
	}
}

// tearDownDlvSession cancels the console bridge and closes the current Delve client.
func (a *DebuggerApp) tearDownDlvSession() {
	a.console.stopBridge()
	a.inferiorIO.unwire()
	a.dlv.clearPendingOnTeardown()
	if db := a.dlvBackend(); db != nil && db.Client != nil {
		db.Client.Close()
		db.Client = nil
	}
}

// ConnectDlv replaces the local `dlv exec` session with `dlv connect <addr>`
// (headless server in another terminal / process). Same-UID localhost only.
func (a *DebuggerApp) ConnectDlv(addr string) error {
	if a.backend == nil || a.backend.Kind() != backend.DLV {
		return fmt.Errorf("dlv_connect: start gdbforge with -g dlv")
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("dlv_connect: address required (e.g. 127.0.0.1:2345)")
	}

	a.tearDownDlvSession()

	client, err := dlv.NewConnectClient(a.cfg.GDBPath, addr)
	if err != nil {
		fallback, ferr := dlv.NewClientOpts(a.cfg.GDBPath, a.cfg.GDBArgs, dlv.ClientOptions{})
		if ferr != nil {
			return fmt.Errorf("dlv connect %s: %w (also failed to restore exec: %v)", addr, err, ferr)
		}
		a.attachRestartedDlv(fallback)
		if inf := fallback.InferiorTTY(); inf != nil {
			a.inferiorIO.rewireInternal(inf)
		}
		return fmt.Errorf("dlv connect failed (%v); restored local dlv exec", err)
	}
	a.attachRestartedDlv(client)
	a.inferiorIO.markExternal(client.InferiorTTYPath())
	// New headless session does not inherit the old local dlv breakpoints —
	// re-apply entry + UI list or `c` will run straight to exit.
	a.breaks.reapplyAfterDlvConnect()
	if a.ctx.Log != nil {
		a.ctx.Log.Named("dlv").Info("connected to headless dlv at " + addr)
	}
	return nil
}

// SessionProgram returns the debuggee path from startup args (first dlv/gdb arg).
func (a *DebuggerApp) SessionProgram() string {
	if a == nil {
		return ""
	}
	args := a.cfg.GDBArgs
	if len(args) == 0 {
		return strings.TrimSpace(a.cfg.Prog)
	}
	// Skip leading gdb-style "--args".
	if args[0] == "--args" && len(args) > 1 {
		return args[1]
	}
	return args[0]
}

// SessionProgramArgs returns program + argv for `dlv exec -- …` (no debugger flags).
func (a *DebuggerApp) SessionProgramArgs() []string {
	if a == nil {
		return nil
	}
	args := append([]string(nil), a.cfg.GDBArgs...)
	if len(args) == 0 {
		if p := strings.TrimSpace(a.cfg.Prog); p != "" {
			return []string{p}
		}
		return nil
	}
	if args[0] == "--args" {
		return args[1:]
	}
	return args
}

// SpawnDlvHeadless opens an external terminal running headless Delve for the
// current session program (plus optional extraArgs for the inferior).
// The window stays open if dlv exits. Listen address is 127.0.0.1:<port>.
func (a *DebuggerApp) SpawnDlvHeadless(port string, extraArgs []string) error {
	port = strings.TrimSpace(port)
	if port == "" {
		port = "2345"
	}
	progArgs := a.SessionProgramArgs()
	if len(progArgs) == 0 {
		return fmt.Errorf("spawn_dlv_headless: no program — start with: gdbforge -g dlv -- ./prog")
	}
	for _, ea := range extraArgs {
		ea = strings.TrimSpace(ea)
		if ea != "" {
			progArgs = append(progArgs, ea)
		}
	}
	dlvBin := a.cfg.GDBPath
	if dlvBin == "" {
		dlvBin = "dlv"
	}
	addr := "127.0.0.1:" + port

	var b strings.Builder
	b.WriteString("echo 'gdbforge headless dlv on ")
	b.WriteString(addr)
	b.WriteString("'; echo 'program:")
	for _, pa := range progArgs {
		b.WriteByte(' ')
		b.WriteString(pa)
	}
	b.WriteString("'; echo;")
	b.WriteString(" ")
	b.WriteString(shellSingleQuote(dlvBin))
	b.WriteString(" exec --headless --listen=")
	b.WriteString(shellSingleQuote(addr))
	// --accept-multiclient allows gdbforge rpc2 + dlv connect to share one session.
	b.WriteString(" --api-version=2 --accept-multiclient --")
	for _, arg := range progArgs {
		b.WriteByte(' ')
		b.WriteString(shellSingleQuote(arg))
	}
	b.WriteString("; ec=$?; echo; echo \"[headless dlv exited $ec — window kept open]\"; exec sleep infinity")

	return a.SpawnTerminal([]string{"sh", "-c", b.String()})
}

func (a *DebuggerApp) closeExternalInferiorHold() {
	if a == nil || a.extInferiorHold == nil {
		return
	}
	h := a.extInferiorHold
	a.extInferiorHold = nil
	// Kill the shell/sleep inside the terminal first — mate-terminal exits with it.
	if h.holdPID > 0 {
		signalProcess(h.holdPID, false, syscall.SIGTERM)
		time.Sleep(150 * time.Millisecond)
		if processAlive(h.holdPID) {
			signalProcess(h.holdPID, false, syscall.SIGKILL)
		}
	}
	if h.termPID > 0 {
		a.children.KillTracked(h.termPID)
	}
}

// externalInferiorHold tracks the terminal emulator and its hold shell so
// :set inferior-tty internal can close the external window.
type externalInferiorHold struct {
	holdPID int // sh/sleep inside the terminal (from pid file)
	termPID int // mate-terminal (etc.) process
}

// OpenExternalTTY opens a real terminal, records its /dev/pts/N for GDB
// -inferior-tty-set, and keeps the window open until internal mode.
func (a *DebuggerApp) OpenExternalTTY() (string, error) {
	a.closeExternalInferiorHold()

	tag := fmt.Sprintf("gdbforge-inf-tty-%d", os.Getpid())
	pathFile := filepath.Join(os.TempDir(), tag)
	pidFile := filepath.Join(os.TempDir(), tag+".pid")
	_ = os.Remove(pathFile)
	_ = os.Remove(pidFile)

	hold := fmt.Sprintf("echo $$ > %s; tty > %s; exec sleep infinity",
		shellSingleQuote(pidFile), shellSingleQuote(pathFile))
	argv, err := terminalHoldArgv(hold)
	if err != nil {
		return "", err
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start terminal %v: %w", argv, err)
	}
	termPID := cmd.Process.Pid
	a.trackStartedCmd(cmd, true)
	go func() { _ = cmd.Wait() }()

	var pts string
	var holdPID int
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if pts == "" {
			if b, err := os.ReadFile(pathFile); err == nil {
				p := strings.TrimSpace(string(b))
				if p != "" && strings.HasPrefix(p, "/dev/") {
					if _, err := os.Stat(p); err == nil {
						pts = p
					}
				}
			}
		}
		if holdPID == 0 {
			if p, err := readPIDFile(pidFile); err == nil {
				holdPID = p
			}
		}
		if pts != "" && holdPID > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pts == "" {
		a.children.KillTracked(termPID)
		return "", fmt.Errorf("timeout waiting for external tty at %s", pathFile)
	}
	a.extInferiorHold = &externalInferiorHold{holdPID: holdPID, termPID: termPID}
	return pts, nil
}

func readPIDFile(path string) (int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return 0, fmt.Errorf("invalid pid in %s", path)
	}
	return pid, nil
}

// SpawnTerminal opens a real terminal emulator running argv (foreground UI
// for gdbserver / dlv headless / the inferior itself).
func (a *DebuggerApp) SpawnTerminal(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("spawn_terminal: need command")
	}
	termArgv, err := terminalRunArgv(argv)
	if err != nil {
		return err
	}
	cmd := exec.Command(termArgv[0], termArgv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn_terminal %v: %w", termArgv, err)
	}
	a.trackStartedCmd(cmd, true)
	go func() { _ = cmd.Wait() }()
	return nil
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func resolveTerminalBin() (bin string, err error) {
	if v := strings.TrimSpace(os.Getenv("GDBFORGE_TERMINAL")); v != "" {
		if _, err := exec.LookPath(v); err != nil {
			return "", fmt.Errorf("GDBFORGE_TERMINAL=%s: %w", v, err)
		}
		return v, nil
	}
	for _, c := range []string{"kitty", "mate-terminal", "gnome-terminal", "xterm", "konsole", "alacritty"} {
		if _, err := exec.LookPath(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no terminal found (install xterm/kitty/mate-terminal or set GDBFORGE_TERMINAL)")
}

// terminalHoldArgv builds argv that opens a terminal running holdShell.
func terminalHoldArgv(holdShell string) ([]string, error) {
	bin, err := resolveTerminalBin()
	if err != nil {
		return nil, err
	}
	switch filepath.Base(bin) {
	case "gnome-terminal":
		return []string{bin, "--", "sh", "-c", holdShell}, nil
	case "mate-terminal":
		// mate-terminal only accepts -e/--command with a single string.
		return []string{bin, "-e", "sh -c " + shellSingleQuote(holdShell)}, nil
	case "xterm":
		return []string{bin, "-e", "sh", "-c", holdShell}, nil
	case "konsole":
		return []string{bin, "-e", "sh", "-c", holdShell}, nil
	default:
		// kitty, alacritty, …
		return []string{bin, "sh", "-c", holdShell}, nil
	}
}

// terminalRunArgv builds argv that opens a terminal running userArgv.
func terminalRunArgv(userArgv []string) ([]string, error) {
	bin, err := resolveTerminalBin()
	if err != nil {
		return nil, err
	}
	switch filepath.Base(bin) {
	case "gnome-terminal":
		out := []string{bin, "--"}
		return append(out, userArgv...), nil
	case "mate-terminal":
		return []string{bin, "-e", "sh -c " + shellSingleQuote(shellJoinArgs(userArgv))}, nil
	case "xterm", "konsole":
		out := []string{bin, "-e"}
		return append(out, userArgv...), nil
	default:
		return append([]string{bin}, userArgv...), nil
	}
}

func shellJoinArgs(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, a := range argv {
		parts = append(parts, shellSingleQuote(a))
	}
	return strings.Join(parts, " ")
}

func inferiorTTYFromEnvOrCfg(cfg SessionConfig) string {
	if p := strings.TrimSpace(cfg.InferiorTTY); p != "" {
		return p
	}
	return strings.TrimSpace(os.Getenv("GDBFORGE_INFERIOR_TTY"))
}
