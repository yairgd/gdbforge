package platform

import tcell "github.com/gdamore/tcell/v2"

// Default theme colors for AppState. Widgets must read colors via AppState
// getters (or these defaults when state is nil) — never hardcode tcell colors
// for themable UI.
var (
	DefaultMarkColor          = tcell.ColorBlue
	DefaultMarkDimColor       = tcell.ColorGray
	DefaultBreakColor         = tcell.ColorRed
	DefaultBreakDisabledColor = tcell.ColorYellow
	// DefaultBreakCondColor is the background for enabled conditional breakpoints.
	DefaultBreakCondColor = tcell.ColorOrange
	// DefaultPCColor is the program-counter / ━━▶ row (Code + Breakpoint list).
	DefaultPCColor = tcell.ColorDarkSlateGray
	// DefaultStackBreakColor highlights a Call Stack frame when ━━▶ is on the
	// same file:line as a breakpoint. Default green.
	DefaultStackBreakColor = tcell.ColorGreen
	// DefaultCodeSelColor is the focused CodeWidget cursor-line background
	// when it is not also the PC line.
	DefaultCodeSelColor = tcell.ColorDarkBlue
	// DefaultMutedColor is dim/empty-list foreground text.
	DefaultMutedColor = tcell.ColorGray
	// DefaultSearchColor is the background for /search match substrings.
	// Dark orange stays visible on both light terminals and dark chroma themes
	// (plain yellow washes out on white). Override with :set searchcolor.
	DefaultSearchColor = tcell.ColorDarkOrange
)
