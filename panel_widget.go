package lobster

type PanelWidget interface {
	// returns struct that will be passed to the panel dashboard template
	Prepare(session *Session) interface{}
}

type PanelWidgetFunc func(*Session) interface{}

func (f PanelWidgetFunc) Prepare(session *Session) interface{} {
	return f(session)
}

var panelWidgets map[string]PanelWidget

func loadPanelWidgets() {
	panelWidgets = make(map[string]PanelWidget)
}

func RegisterPanelWidget(name string, widget PanelWidget) {
	panelWidgets[name] = widget
}
