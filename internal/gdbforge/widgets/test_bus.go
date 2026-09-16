package widgets

import "github.com/yairgd/termforge/platform"

func testWidgetCtx() platform.AppContext {
	return platform.AppContext{Bus: platform.NewEventBus()}
}
