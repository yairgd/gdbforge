package widgets

import "github.com/yairgd/gdbforge/internal/platform"

func testWidgetCtx() platform.AppContext {
	return platform.AppContext{Bus: platform.NewEventBus()}
}
