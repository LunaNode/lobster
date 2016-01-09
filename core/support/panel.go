package support

import "github.com/LunaNode/lobster"

import "github.com/gorilla/mux"

import "fmt"
import "net/http"
import "strconv"

type SupportParams struct {
	Frame   lobster.FrameParams
	Tickets []*Ticket
}

func panelSupport(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	params := SupportParams{}
	params.Frame = frameParams
	params.Tickets = TicketList(db, session.UserId)
	lobster.RenderTemplate(w, "panel", "support", params)
}

type SupportOpenForm struct {
	Name    string `schema:"name"`
	Message string `schema:"message"`
}

func panelSupportOpen(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	if r.Method == "POST" {
		form := new(SupportOpenForm)
		err := decoder.Decode(form, r.PostForm)
		if err != nil {
			http.Redirect(w, r, "/panel/support/open", 303)
			return
		}

		ticketId, err := ticketOpen(db, session.UserId, form.Name, form.Message, false)
		if err != nil {
			lobster.RedirectMessage(w, r, "/panel/support/open", L.FormatError(err))
		} else {
			lobster.LogAction(db, session.UserId, lobster.ExtractIP(r.RemoteAddr), "Open ticket", fmt.Sprintf("Subject: %s; Ticket ID: %d", form.Name, ticketId))
			http.Redirect(w, r, fmt.Sprintf("/panel/support/%d", ticketId), 303)
		}
		return
	}

	lobster.RenderTemplate(w, "panel", "support_open", lobster.PanelFormParams{Frame: frameParams, Token: lobster.CSRFGenerate(db, session)})
}

type PanelSupportTicketParams struct {
	Frame  lobster.FrameParams
	Ticket *Ticket
	Token  string
}

func panelSupportTicket(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	ticketId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		lobster.RedirectMessage(w, r, "/panel/support", L.FormattedError("invalid_ticket"))
		return
	}
	ticket := TicketDetails(db, session.UserId, ticketId, false)
	if ticket == nil {
		lobster.RedirectMessage(w, r, "/panel/support", L.FormattedError("ticket_not_found"))
		return
	}

	params := PanelSupportTicketParams{}
	params.Frame = frameParams
	params.Ticket = ticket
	params.Token = lobster.CSRFGenerate(db, session)
	lobster.RenderTemplate(w, "panel", "support_ticket", params)
}

type SupportTicketReplyForm struct {
	Message string `schema:"message"`
}

func panelSupportTicketReply(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	ticketId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		lobster.RedirectMessage(w, r, "/panel/support", L.FormattedError("invalid_ticket"))
		return
	}
	form := new(SupportTicketReplyForm)
	err = decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/panel/support/%d", ticketId), 303)
		return
	}

	err = ticketReply(db, session.UserId, ticketId, form.Message, false)
	if err != nil {
		lobster.RedirectMessage(w, r, fmt.Sprintf("/panel/support/%d", ticketId), L.FormatError(err))
	} else {
		lobster.LogAction(db, session.UserId, lobster.ExtractIP(r.RemoteAddr), "Ticket reply", fmt.Sprintf("Ticket ID: %d", ticketId))
		http.Redirect(w, r, fmt.Sprintf("/panel/support/%d", ticketId), 303)
	}
}

func panelSupportTicketClose(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	ticketId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		lobster.RedirectMessage(w, r, "/panel/support", L.FormattedError("invalid_ticket"))
		return
	}
	ticketClose(db, session.UserId, ticketId)
	lobster.LogAction(db, session.UserId, lobster.ExtractIP(r.RemoteAddr), "Close ticket", fmt.Sprintf("Ticket ID: %d", ticketId))
	lobster.RedirectMessage(w, r, fmt.Sprintf("/panel/support/%d", ticketId), L.Success("ticket_closed"))
}
