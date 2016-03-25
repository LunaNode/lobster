package lobster

import "github.com/gorilla/mux"

import "fmt"
import "net/http"
import "strconv"

type AdminFormParams struct {
	Frame FrameParams
	Token string
}

type AdminHandlerFunc func(http.ResponseWriter, *http.Request, *Session, FrameParams)

func adminWrap(h AdminHandlerFunc) func(http.ResponseWriter, *http.Request, *Session) {
	return func(w http.ResponseWriter, r *http.Request, session *Session) {
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
		user := UserDetails(session.UserId)
		if !user.Admin || !session.Admin {
			http.Redirect(w, r, "/panel/dashboard", 303)
			return
		}

		var frameParams FrameParams
		if r.URL.Query()["message"] != nil {
			frameParams.Message.Text = r.URL.Query()["message"][0]
			if r.URL.Query()["type"] != nil {
				frameParams.Message.Type = r.URL.Query()["type"][0]
			} else {
				frameParams.Message.Type = "info"
			}
		}
		h(w, r, session, frameParams)
	}
}

func adminDashboard(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	RenderTemplate(w, "admin", "dashboard", frameParams)
}

type AdminUsersParams struct {
	Frame FrameParams
	Users []*User
}

func adminUsers(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	params := AdminUsersParams{}
	params.Frame = frameParams
	params.Users = UserList()
	RenderTemplate(w, "admin", "users", params)
}

type AdminUserProcessFunc func(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, user *User)

func adminUserProcess(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, f AdminUserProcessFunc) {
	userId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		RedirectMessage(w, r, "/admin/users", L.FormattedError("invalid_user"))
		return
	}
	user := UserDetails(userId)
	if user == nil {
		RedirectMessage(w, r, "/admin/users", L.FormattedError("user_not_found"))
		return
	} else {
		f(w, r, session, frameParams, user)
	}
}

type AdminUserParams struct {
	Frame           FrameParams
	User            *User
	StatusAction    string // action that admin can take on this user, either "disable" or "enable" depending on current user status
	VirtualMachines []*VirtualMachine
	Token           string
}

func adminUser(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	adminUserProcess(w, r, session, frameParams, func(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, user *User) {
		params := AdminUserParams{}
		params.Frame = frameParams
		params.User = user
		if user.Status == "disabled" {
			params.StatusAction = "enable"
		} else {
			params.StatusAction = "disable"
		}
		params.VirtualMachines = vmList(user.Id)
		params.Token = CSRFGenerate(session)
		RenderTemplate(w, "admin", "user", params)
	})
}

func adminUserLogin(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	adminUserProcess(w, r, session, frameParams, func(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, user *User) {
		session.OriginalId = session.UserId
		session.UserId = user.Id
		http.Redirect(w, r, "/panel/dashboard", 303)
	})
}

type AdminUserCreditForm struct {
	Credit      float64 `schema:"credit"`
	Description string  `schema:"description"`
}

func adminUserCredit(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	adminUserProcess(w, r, session, frameParams, func(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, user *User) {
		form := new(AdminUserCreditForm)
		err := decoder.Decode(form, r.PostForm)
		if err != nil {
			http.Redirect(w, r, fmt.Sprintf("/admin/user/%d", user.Id), 303)
			return
		}

		creditInt := int64(form.Credit * BILLING_PRECISION)
		UserApplyCredit(user.Id, creditInt, form.Description)
		RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", user.Id), L.Success("credit_applied"))
	})
}

func adminUserPassword(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	adminUserProcess(w, r, session, frameParams, func(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, user *User) {
		if r.PostFormValue("password") != r.PostFormValue("password_confirm") {
			RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", user.Id), L.FormattedError("password_mismatch"))
			return
		} else if r.PostFormValue("password") == "" {
			RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", user.Id), L.FormattedError("password_empty"))
			return
		}

		authForceChangePassword(user.Id, r.PostFormValue("password"))
		RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", user.Id), L.Success("password_reset"))
	})
}

func adminUserDisable(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	adminUserProcess(w, r, session, frameParams, func(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, user *User) {
		db.Exec("UPDATE users SET status = 'disabled' WHERE id = ?", user.Id)
		RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", user.Id), L.Success("user_disabled"))
	})
}

func adminUserEnable(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	adminUserProcess(w, r, session, frameParams, func(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams, user *User) {
		db.Exec("UPDATE users SET status = 'active' WHERE id = ?", user.Id)
		RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", user.Id), L.Success("user_enabled"))
	})
}

type AdminVirtualMachinesParams struct {
	Frame           FrameParams
	VirtualMachines []*VirtualMachine
}

func adminVirtualMachines(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	params := AdminVirtualMachinesParams{}
	params.Frame = frameParams
	params.VirtualMachines = vmListAll()
	RenderTemplate(w, "admin", "vms", params)
}

// virtual machine actions
func adminVMProcess(r *http.Request) (*VirtualMachine, error) {
	vmId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		return nil, fmt.Errorf("invalid VM ID")
	}
	vm := vmGet(vmId)
	if vm == nil {
		return nil, fmt.Errorf("VM does not exist")
	}
	return vm, nil
}

func adminVMSuspend(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	vm, err := adminVMProcess(r)
	if err != nil {
		RedirectMessage(w, r, "/admin/users", L.FormatError(err))
	}
	vm.Suspend(false)
	RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", vm.UserId), L.Success("vm_suspended"))
}

func adminVMUnsuspend(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	vm, err := adminVMProcess(r)
	if err != nil {
		RedirectMessage(w, r, "/admin/users", L.FormatError(err))
	}
	err = vm.Unsuspend()
	if err != nil {
		RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", vm.UserId), L.FormatError(err))
	} else {
		RedirectMessage(w, r, fmt.Sprintf("/admin/user/%d", vm.UserId), L.Success("vm_unsuspended"))
	}
}

type AdminPlansParams struct {
	Frame   FrameParams
	Plans   []*Plan
	Regions []string
	Token   string
}

func adminPlans(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	params := AdminPlansParams{}
	params.Frame = frameParams
	params.Plans = planList()
	params.Regions = regionList()
	params.Token = CSRFGenerate(session)
	RenderTemplate(w, "admin", "plans", params)
}

type AdminPlansAddForm struct {
	Name      string  `schema:"name"`
	Price     float64 `schema:"price"`
	Ram       int     `schema:"ram"`
	Cpu       int     `schema:"cpu"`
	Storage   int     `schema:"storage"`
	Bandwidth int     `schema:"bandwidth"`
	Global    string  `schema:"global"`
}

func adminPlansAdd(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	form := new(AdminPlansAddForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		RedirectMessage(w, r, "/admin/plans", L.FormatError(err))
		return
	}

	planCreate(form.Name, int64(form.Price*BILLING_PRECISION), form.Ram, form.Cpu, form.Storage, form.Bandwidth, form.Global != "")
	RedirectMessage(w, r, "/admin/plans", L.Success("plan_created"))
}

func adminPlansAutopopulate(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	err := planAutopopulate(r.PostFormValue("region"))
	if err != nil {
		RedirectMessage(w, r, "/admin/plans", L.FormatError(err))
		return
	} else {
		RedirectMessage(w, r, "/admin/plans", L.Success("plan_autopopulate_success"))
	}
}

type AdminPlanParams struct {
	Frame   FrameParams
	Plan    *Plan
	Regions []string
	Token   string
}

func adminPlan(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	planId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		RedirectMessage(w, r, "/admin/plans", L.FormattedError("invalid_plan"))
		return
	}
	plan := planGet(planId)
	if plan == nil {
		RedirectMessage(w, r, "/admin/plans", L.FormattedError("plan_not_found"))
		return
	}
	plan.LoadRegionPlans()
	params := AdminPlanParams{}
	params.Frame = frameParams
	params.Plan = plan
	params.Regions = regionList()
	params.Token = CSRFGenerate(session)
	RenderTemplate(w, "admin", "plan", params)
}

func adminPlanDelete(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	planId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		RedirectMessage(w, r, "/admin/plans", L.FormattedError("invalid_plan"))
		return
	}
	planDelete(planId)
	RedirectMessage(w, r, "/admin/plans", L.Success("plan_deleted"))
}

func adminPlanAssociateRegion(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	planId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		RedirectMessage(w, r, "/admin/plans", L.FormattedError("invalid_plan"))
		return
	}
	err = planAssociateRegion(planId, r.PostFormValue("region"), r.PostFormValue("identification"))
	if err != nil {
		RedirectMessage(w, r, fmt.Sprintf("/admin/plan/%d", planId), L.FormatError(err))
	} else {
		RedirectMessage(w, r, fmt.Sprintf("/admin/plan/%d", planId), L.Success("plan_region_associated"))
	}
}

func adminPlanDeassociateRegion(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	planId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		RedirectMessage(w, r, "/admin/plans", L.FormattedError("invalid_plan"))
		return
	}
	planDeassociateRegion(planId, mux.Vars(r)["region"])
	RedirectMessage(w, r, fmt.Sprintf("/admin/plan/%d", planId), L.Success("plan_region_deassociated"))
}

type AdminImagesParams struct {
	Frame   FrameParams
	Images  []*Image
	Regions []string
	Token   string
}

func adminImages(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	params := AdminImagesParams{}
	params.Frame = frameParams
	params.Images = imageListAll()
	params.Regions = regionList()
	params.Token = CSRFGenerate(session)
	RenderTemplate(w, "admin", "images", params)
}

type AdminImagesAddForm struct {
	Name           string `schema:"name"`
	Region         string `schema:"region"`
	Identification string `schema:"identification"`
}

func adminImagesAdd(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	form := new(AdminImagesAddForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/admin/images", 303)
		return
	}

	imageAdd(form.Name, form.Region, form.Identification)
	RedirectMessage(w, r, "/admin/images", L.Success("image_added"))
}

func adminImageDelete(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	imageId, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		RedirectMessage(w, r, "/admin/images", L.FormattedError("invalid_plan"))
		return
	}

	err = imageDeleteForce(imageId)
	if err != nil {
		RedirectMessage(w, r, "/admin/images", L.FormatError(err))
	} else {
		RedirectMessage(w, r, "/admin/images", L.Success("image_deleted"))
	}
}

func adminImagesAutopopulate(w http.ResponseWriter, r *http.Request, session *Session, frameParams FrameParams) {
	err := imageAutopopulate(r.PostFormValue("region"))
	if err != nil {
		RedirectMessage(w, r, "/admin/images", L.FormatError(err))
		return
	} else {
		RedirectMessage(w, r, "/admin/images", L.Success("image_autopopulate_success"))
	}
}
