package lobster

import "github.com/gorilla/mux"

import "fmt"
import "net/http"
import "strconv"

type AdminFormParams struct {
	Frame FrameParams
	Token string
}

type AdminHandlerFunc func(http.ResponseWriter, *http.Request, *Database, *Session, FrameParams)

func adminWrap(h AdminHandlerFunc) func(http.ResponseWriter, *http.Request, *Database, *Session) {
	return func(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
		if !session.IsLoggedIn() {
			http.Redirect(w, r, "/login", 303)
			return
		}

		// revert login as another user
		if session.OriginalId != 0 {
			session.UserId = session.OriginalId
			session.OriginalId = 0
		}

		// confirm session admin and also user still admin
		user := userDetails(db, session.UserId)
		if !user.Admin || !session.Admin {
			http.Redirect(w, r, "/panel/dashboard", 303)
			return
		}

		var frameParams FrameParams
		if r.URL.Query()["message"] != nil {
			frameParams.Message = r.URL.Query()["message"][0]
		}
		h(w, r, db, session, frameParams)
	}
}

func adminDashboard(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	renderTemplate(w, "admin", "dashboard", frameParams)
}

type AdminUsersParams struct {
	Frame FrameParams
	Users []*User
}
func adminUsers(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := AdminUsersParams{}
	params.Frame = frameParams
	params.Users = userList(db)
	renderTemplate(w, "admin", "users", params)
}

type AdminUserParams struct {
	Frame FrameParams
	User *User
	VirtualMachines []*VirtualMachine
	Token string
}
func adminUser(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Redirect(w, r, "/panel/users", 303)
		return
	}
	params := AdminUserParams{}
	params.Frame = frameParams
	params.User = userDetails(db, int(userId))
	params.VirtualMachines = vmList(db, int(userId))
	params.Token = csrfGenerate(db, session)
	renderTemplate(w, "admin", "user", params)
}


func adminUserLogin(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Redirect(w, r, "/panel/users", 303)
	} else {
		session.OriginalId = session.UserId
		session.UserId = int(userId)
		http.Redirect(w, r, "/panel/dashboard", 303)
	}
}

func adminSupportTicketClose(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Redirect(w, r, "/panel/support", 303)
		return
	}
	ticketClose(db, session.UserId, int(ticketId))
	LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Close ticket", fmt.Sprintf("Ticket ID: %d", ticketId))
	redirectMessage(w, r, fmt.Sprintf("/panel/support/%d", ticketId), "This ticket has been marked closed.")
}

type AdminSupportParams struct {
	Frame FrameParams
	Tickets []*Ticket
}
func adminSupport(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := AdminSupportParams{}
	params.Frame = frameParams
	params.Tickets = ticketListAll(db)
	renderTemplate(w, "admin", "support", params)
}

type AdminSupportOpenParams struct {
	Frame FrameParams
	Token string
	UserId int
}
type AdminSupportOpenForm struct {
	UserId int `schema:"user_id"`
	Name string `schema:"name"`
	Message string `schema:"message"`
}
func adminSupportOpen(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Redirect(w, r, "/admin/support", 303)
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
			redirectMessage(w, r, fmt.Sprintf("/admin/support/open/%d", userId), "Error: " + err.Error() + ".")
		} else {
			http.Redirect(w, r, fmt.Sprintf("/admin/support/%d", ticketId), 303)
		}
		return
	}

	renderTemplate(w, "admin", "support_open", AdminSupportOpenParams{Frame: frameParams, Token: csrfGenerate(db, session), UserId: int(userId)})
}

type AdminSupportTicketParams struct {
	Frame FrameParams
	Ticket *Ticket
	Token string
}
func adminSupportTicket(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Redirect(w, r, "/admin/support", 303)
		return
	}
	ticket := ticketDetails(db, session.UserId, int(ticketId), true)
	if ticket == nil {
		http.Redirect(w, r, "/admin/support", 303)
		return
	}

	params := AdminSupportTicketParams{}
	params.Frame = frameParams
	params.Ticket = ticket
	params.Token = csrfGenerate(db, session)
	renderTemplate(w, "admin", "support_ticket", params)
}

type AdminSupportTicketReplyForm struct {
	Message string `schema:"message"`
}
func adminSupportTicketReply(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Redirect(w, r, "/admin/support", 303)
		return
	}
	form := new(AdminSupportTicketReplyForm)
	err = decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/admin/support/%d", ticketId), 303)
		return
	}

	err = ticketReply(db, session.UserId, int(ticketId), form.Message, true)
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/admin/support/%d", ticketId), "Error: " + err.Error() + ".")
	} else {
		http.Redirect(w, r, fmt.Sprintf("/admin/support/%d", ticketId), 303)
	}
}
