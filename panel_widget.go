package lobster

type PanelWidget interface {
	// returns struct that will be passed to the panel dashboard template
	Prepare(db *Database, session *Session) interface{}
}

type PanelWidgetFunc func(*Database, *Session) interface{}

func (f PanelWidgetFunc) Prepare(db *Database, session *Session) interface{} {
	return f(db, session)
}

var panelWidgets map[string]PanelWidget

func loadPanelWidgets() {
	panelWidgets = make(map[string]PanelWidget)
}

func RegisterPanelWidget(name string, widget PanelWidget) {
	panelWidgets[name] = widget
}
