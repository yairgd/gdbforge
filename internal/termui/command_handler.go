package termui

type CommandHandler func(ev CommandEvent) bool

type CommandHandlers map[CommandID]CommandHandler
