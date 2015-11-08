package support

import "github.com/LunaNode/lobster"

import "github.com/gorilla/mux"

import "fmt"
import "net/http"
import "strconv"

func adminSupportTicketClose(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	ticketId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		lobster.RedirectMessage(w, r, "/admin/support", L.FormattedError("invalid_ticket"))
		return
	}
	ticketClose(db, session.UserId, ticketId)
	lobster.LogAction(db, session.UserId, lobster.ExtractIP(r.RemoteAddr), "Close ticket", fmt.Sprintf("Ticket ID: %d", ticketId))
	lobster.RedirectMessage(w, r, fmt.Sprintf("/admin/support/%d", ticketId), L.Success("ticket_closed"))
}

type AdminSupportParams struct {
	Frame lobster.FrameParams
	Tickets []*Ticket
}
func adminSupport(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	params := AdminSupportParams{}
	params.Frame = frameParams
	params.Tickets = TicketListAll(db)
	lobster.RenderTemplate(w, "admin", "support", params)
}

type AdminSupportOpenParams struct {
	Frame lobster.FrameParams
	User *lobster.User
	Token string
}
type AdminSupportOpenForm struct {
	UserId int `schema:"user_id"`
	Name string `schema:"name"`
	Message string `schema:"message"`
}
func adminSupportOpen(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	userId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		lobster.RedirectMessage(w, r, "/admin/support", L.FormattedError("invalid_user"))
		return
	}
	user := lobster.UserDetails(db, userId)
	if user == nil {
		lobster.RedirectMessage(w, r, "/admin/support", L.FormattedError("user_not_found"))
		return
	}

	if r.Method == "POST" {
		form := new(AdminSupportOpenForm)
		err := decoder.Decode(form, r.PostForm)
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/support/open/%d", userId), 303)
			return
		}

		ticketId, err := ticketOpen(db, form.UserId, form.Name, form.Message, true)
		if err != nil {
			lobster.RedirectMessage(w, r, fmt.Sprintf("/admin/support/open/%d", userId), L.FormatError(err))
		} else {
			http.Redirect(w, r, fmt.Sprintf("/admin/support/%d", ticketId), 303)
		}
		return
	}

	params := new(AdminSupportOpenParams)
	params.Frame = frameParams
	params.User = user
	params.Token = lobster.CSRFGenerate(db, session)
	lobster.RenderTemplate(w, "admin", "support_open", params)
}

type AdminSupportTicketParams struct {
	Frame lobster.FrameParams
	Ticket *Ticket
	Token string
}
func adminSupportTicket(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	ticketId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		lobster.RedirectMessage(w, r, "/admin/support", L.FormattedError("invalid_ticket"))
		return
	}
	ticket := TicketDetails(db, session.UserId, ticketId, true)
	if ticket == nil {
		lobster.RedirectMessage(w, r, "/admin/support", L.FormattedError("ticket_not_found"))
		return
	}

	params := AdminSupportTicketParams{}
	params.Frame = frameParams
	params.Ticket = ticket
	params.Token = lobster.CSRFGenerate(db, session)
	lobster.RenderTemplate(w, "admin", "support_ticket", params)
}

type AdminSupportTicketReplyForm struct {
	Message string `schema:"message"`
}
func adminSupportTicketReply(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	ticketId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		lobster.RedirectMessage(w, r, "/admin/support", L.FormattedError("invalid_ticket"))
		return
	}
	form := new(AdminSupportTicketReplyForm)
	err = decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/admin/support/%d", ticketId), 303)
		return
	}

	err = ticketReply(db, session.UserId, ticketId, form.Message, true)
	if err != nil {
		lobster.RedirectMessage(w, r, fmt.Sprintf("/admin/support/%d", ticketId), L.FormatError(err))
	} else {
		http.Redirect(w, r, fmt.Sprintf("/admin/support/%d", ticketId), 303)
	}
}
