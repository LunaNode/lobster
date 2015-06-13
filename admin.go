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
		redirectMessage(w, r, "/admin/users", "Error: invalid user ID.")
		return
	}
	user := userDetails(db, int(userId))
	if user == nil {
		redirectMessage(w, r, "/admin/users", "Error: user not found.")
		return
	}
	params := AdminUserParams{}
	params.Frame = frameParams
	params.User = user
	params.VirtualMachines = vmList(db, int(userId))
	params.Token = csrfGenerate(db, session)
	renderTemplate(w, "admin", "user", params)
}

func adminUserLogin(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/users", "Error: invalid user ID.")
	} else {
		session.OriginalId = session.UserId
		session.UserId = int(userId)
		http.Redirect(w, r, "/panel/dashboard", 303)
	}
}

type AdminUserCreditForm struct {
	Credit float64 `schema:"credit"`
	Description string `schema:"description"`
}
func adminUserCredit(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/users", "Error: invalid user ID.")
		return
	}
	form := new(AdminUserCreditForm)
	err = decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/admin/user/%d", userId), 303)
		return
	}

	creditInt := int64(form.Credit * BILLING_PRECISION)
	userApplyCredit(db, int(userId), creditInt, form.Description)
	redirectMessage(w, r, fmt.Sprintf("/admin/user/%d", userId), "Credit applied successfully.")
}

func adminUserPassword(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/users", "Error: invalid user ID.")
		return
	} else if r.PostFormValue("password") != r.PostFormValue("password_confirm") {
		redirectMessage(w, r, fmt.Sprintf("/admin/user/%d", userId), "Error: passwords do not match.")
		return
	} else if r.PostFormValue("password") == "" {
		redirectMessage(w, r, fmt.Sprintf("/admin/user/%d", userId), "Error: password cannot be empty.")
		return
	}

	authForceChangePassword(db, int(userId), r.PostFormValue("password"))
	redirectMessage(w, r, fmt.Sprintf("/admin/user/%d", userId), "Password reset successfully.")
}

func adminUserDisable(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/users", "Error: invalid user ID.")
	} else {
		db.Exec("UPDATE users SET status = 'disabled' WHERE id = ?", userId)
		redirectMessage(w, r, fmt.Sprintf("/admin/user/%d", userId), "Account disabled successfully.")
	}
}

func adminSupportTicketClose(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/support", "Error: invalid ticket ID.")
		return
	}
	ticketClose(db, session.UserId, int(ticketId))
	LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Close ticket", fmt.Sprintf("Ticket ID: %d", ticketId))
	redirectMessage(w, r, fmt.Sprintf("/admin/support/%d", ticketId), "This ticket has been marked closed.")
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
	User *User
	Token string
}
type AdminSupportOpenForm struct {
	UserId int `schema:"user_id"`
	Name string `schema:"name"`
	Message string `schema:"message"`
}
func adminSupportOpen(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	userId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/support", "Error: invalid user ID.")
		return
	}
	user := userDetails(db, int(userId))
	if user == nil {
		redirectMessage(w, r, "/admin/support", "Error: user not found.")
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

	params := new(AdminSupportOpenParams)
	params.Frame = frameParams
	params.User = user
	params.Token = csrfGenerate(db, session)
	renderTemplate(w, "admin", "support_open", params)
}

type AdminSupportTicketParams struct {
	Frame FrameParams
	Ticket *Ticket
	Token string
}
func adminSupportTicket(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/support", "Error: invalid ticket ID.")
		return
	}
	ticket := ticketDetails(db, session.UserId, int(ticketId), true)
	if ticket == nil {
		redirectMessage(w, r, "/admin/support", "Error: ticket not found.")
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
		redirectMessage(w, r, "/admin/support", "Error: invalid ticket ID.")
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

type AdminPlansParams struct {
	Frame FrameParams
	Plans []*Plan
	Token string
}
func adminPlans(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := AdminPlansParams{}
	params.Frame = frameParams
	params.Plans = planList(db)
	params.Token = csrfGenerate(db, session)
	renderTemplate(w, "admin", "plans", params)
}

type AdminPlansAddForm struct {
	Name string `schema:"name"`
	Price float64 `schema:"price"`
	Ram int `schema:"ram"`
	Cpu int `schema:"cpu"`
	Storage int `schema:"storage"`
	Bandwidth int `schema:"bandwidth"`
}
func adminPlansAdd(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	form := new(AdminPlansAddForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		redirectMessage(w, r, "/admin/plans", "Error: " + err.Error() + ".")
		return
	}

	planCreate(db, form.Name, int64(form.Price * BILLING_PRECISION), form.Ram, form.Cpu, form.Storage, form.Bandwidth)
	redirectMessage(w, r, "/admin/plans", "Plan created successfully.")
}

func adminPlanDelete(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	planId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/plans", "Error: invalid plan ID.")
		return
	}
	planDelete(db, int(planId))
	redirectMessage(w, r, "/admin/plans", "Plan deleted successfully.")
}

type AdminImagesParams struct {
	Frame FrameParams
	Images []*Image
	Regions []string
	Token string
}
func adminImages(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := AdminImagesParams{}
	params.Frame = frameParams
	params.Images = imageListAll(db)
	params.Regions = regionList()
	params.Token = csrfGenerate(db, session)
	renderTemplate(w, "admin", "images", params)
}

type AdminImagesAddForm struct {
	Name string `schema:"name"`
	Region string `schema:"region"`
	Identification string `schema:"identification"`
}
func adminImagesAdd(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	form := new(AdminImagesAddForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/admin/images", 303)
		return
	}

	imageAdd(db, form.Name, form.Region, form.Identification)
	redirectMessage(w, r, "/admin/images", "Image added successfully.")
}

func adminImageDelete(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	imageId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/admin/images", "Error: invalid image ID.")
		return
	}

	err = imageDeleteForce(db, int(imageId))
	if err != nil {
		redirectMessage(w, r, "/admin/images", "Error: " + err.Error() + ".")
	} else {
		redirectMessage(w, r, "/admin/images", "Image deleted.")
	}
}
