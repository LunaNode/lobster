package support

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/i18n"

import "github.com/gorilla/schema"

type TicketUpdateEmail struct {
	Id      int
	Subject string
	Message string
}

var decoder *schema.Decoder
var L *i18n.Section
var cfg *lobster.Config

func Setup() {
	decoder = lobster.GetDecoder()
	L = lobster.L
	cfg = lobster.GetConfig()

	lobster.RegisterPanelHandler("/panel/support", panelSupport, false)
	lobster.RegisterPanelHandler("/panel/support/open", panelSupportOpen, false)
	lobster.RegisterPanelHandler("/panel/support/{id:[0-9]+}", panelSupportTicket, false)
	lobster.RegisterPanelHandler("/panel/support/{id:[0-9]+}/reply", panelSupportTicketReply, true)
	lobster.RegisterPanelHandler("/panel/support/{id:[0-9]+}/close", panelSupportTicketClose, true)

	lobster.RegisterAdminHandler("/admin/support", adminSupport, false)
	lobster.RegisterAdminHandler("/admin/support/open/{id:[0-9]+}", adminSupportOpen, false)
	lobster.RegisterAdminHandler("/admin/support/{id:[0-9]+}", adminSupportTicket, false)
	lobster.RegisterAdminHandler("/admin/support/{id:[0-9]+}/reply", adminSupportTicketReply, true)
	lobster.RegisterAdminHandler("/admin/support/{id:[0-9]+}/close", adminSupportTicketClose, true)

	lobster.RegisterPanelWidget("Support", lobster.PanelWidgetFunc(func(db *lobster.Database, session *lobster.Session) interface{} {
		return TicketListActive(db, session.UserId)
	}))
}
