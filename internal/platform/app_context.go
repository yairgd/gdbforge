package platform

type AppContext struct {
	Bus *EventBus
	Log *Logger
}

func NewAppContext() AppContext {
	return AppContext{
		Bus: NewEventBus(),
		Log: NewLogger(),
	}
}
