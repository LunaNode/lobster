package lobster

import "github.com/LunaNode/lobster/utils"

import "github.com/gorilla/mux"

import "errors"
import "fmt"
import "net/http"
import "strconv"
import "time"

type FrameParams struct {
	Message utils.Message
	Error bool
	UserId int
	Admin bool
	OriginalId int // non-zero only if admin is logged in as another user
	Styles []string // additional CSS
	Scripts []string // additional JS
}
type PanelFormParams struct {
	Frame FrameParams
	Token string
}

type PanelHandlerFunc func(http.ResponseWriter, *http.Request, *Database, *Session, FrameParams)

func panelWrap(h PanelHandlerFunc) func(http.ResponseWriter, *http.Request, *Database, *Session) {
	return func(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
		if !session.IsLoggedIn() {
			http.Redirect(w, r, "/login", 303)
		} else {
			var frameParams = FrameParams{
				UserId: session.UserId,
				Admin: session.Admin,
				OriginalId: session.OriginalId,
			}
			if r.URL.Query()["message"] != nil {
				frameParams.Message.Text = r.URL.Query()["message"][0]
				if r.URL.Query()["type"] != nil {
					frameParams.Message.Type = r.URL.Query()["type"][0]
				} else {
					frameParams.Message.Type = "info"
				}
			}
			h(w, r, db, session, frameParams)
		}
	}
}

type PanelDashboardParams struct {
	Frame FrameParams
	VirtualMachines []*VirtualMachine
	CreditSummary *CreditSummary
	BandwidthSummary map[string]*BandwidthSummary
	Tickets []*Ticket
}
func panelDashboard(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := PanelDashboardParams{}
	params.Frame = frameParams
	params.VirtualMachines = vmList(db, session.UserId)
	params.CreditSummary = userCreditSummary(db, session.UserId)
	params.BandwidthSummary = userBandwidthSummary(db, session.UserId)
	params.Tickets = ticketListActive(db, session.UserId)
	RenderTemplate(w, "panel", "dashboard", params)
}

type PanelVirtualMachinesParams struct {
	Frame FrameParams
	VirtualMachines []*VirtualMachine
}
func panelVirtualMachines(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := PanelVirtualMachinesParams{}
	params.Frame = frameParams
	params.VirtualMachines = vmList(db, session.UserId)
	RenderTemplate(w, "panel", "vms", params)
}

type PanelNewVMParams struct {
	Frame FrameParams
	Regions []string
}
func panelNewVM(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := PanelNewVMParams{}
	params.Frame = frameParams
	params.Regions = regionList()
	RenderTemplate(w, "panel", "newvm", params)
}

type PanelNewVMRegionParams struct {
	Frame FrameParams
	Region string
	PublicImages []*Image
	UserImages []*Image
	Plans []*Plan
	Token string
}
type NewVMRegionForm struct {
	Name string `schema:"name"`
	PlanId int `schema:"plan_id"`
	ImageId int `schema:"image_id"`
}
func panelNewVMRegion(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	region := mux.Vars(r)["region"]

	if r.Method == "POST" {
		form := new(NewVMRegionForm)
		err := decoder.Decode(form, r.PostForm)
		if err != nil {
			http.Redirect(w, r, "/panel/newvm/" + region, 303)
			return
		}

		vmId, err := vmCreate(db, session.UserId, form.Name, form.PlanId, form.ImageId)
		if err != nil {
			redirectMessage(w, r, "/panel/newvm/" + region, L.FormatError(err))
		} else {
			LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Create VM", fmt.Sprintf("Name: %s, Plan: %d, Image: %d", form.Name, form.PlanId, form.ImageId))
			http.Redirect(w, r, fmt.Sprintf("/panel/vm/%d", vmId), 303)
		}
		return
	}

	params := PanelNewVMRegionParams{}
	params.Frame = frameParams
	params.Region = region
	params.Plans = planList(db)
	params.Token = csrfGenerate(db, session)

	for _, image := range imageListRegion(db, session.UserId, region) {
		if image.UserId == -1 {
			params.PublicImages = append(params.PublicImages, image)
		} else {
			params.UserImages = append(params.UserImages, image)
		}
	}

	RenderTemplate(w, "panel", "newvm_region", params)
}

type PanelVMParams struct {
	Frame FrameParams
	Vm *VirtualMachine
	Images []*Image
	Plans []*Plan
	Token string
}
func panelVM(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormattedError("invalid_vm"))
		return
	}
	vm := vmGetUser(db, session.UserId, int(vmId))
	if vm == nil {
		redirectMessage(w, r, "/panel/vms", L.FormattedError("vm_not_found"))
		return
	}
	vm.LoadInfo()

	frameParams.Styles = []string{"ladda"}
	frameParams.Scripts = []string{"spin", "ladda", "lobstervm"}
	params := PanelVMParams{}
	params.Frame = frameParams
	params.Vm = vm
	params.Images = imageListRegion(db, session.UserId, vm.Region)
	params.Plans = planList(db)
	params.Token = csrfGenerate(db, session)
	RenderTemplate(w, "panel", "vm", params)
}

// virtual machine actions
func panelVMProcess(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) (*VirtualMachine, error) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		return nil, errors.New("invalid VM ID")
	}
	vm := vmGetUser(db, session.UserId, int(vmId))
	if vm == nil {
		return nil, errors.New("VM does not exist")
	}
	return vm, nil
}

func panelVMStart(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}
	err = vm.Start()
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Start VM", fmt.Sprintf("VM ID: %d", vm.Id))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.Success("vm_started"))
	}
}
func panelVMStop(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}
	err = vm.Stop()
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Stop VM", fmt.Sprintf("VM ID: %d", vm.Id))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.Success("vm_stopped"))
	}
}
func panelVMReboot(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}
	err = vm.Reboot()
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Reboot VM", fmt.Sprintf("VM ID: %d", vm.Id))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.Success("vm_rebooted"))
	}
}
func panelVMDelete(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}
	err = vm.Delete(session.UserId)
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Delete VM", fmt.Sprintf("VM ID: %d", vm.Id))
		redirectMessage(w, r, "/panel/vms", L.Success("vm_deleted"))
	}
}
func panelVMAction(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}

	action := mux.Vars(r)["action"]
	err = vm.Action(action, r.PostFormValue("value"))
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "VM action", fmt.Sprintf("VM ID: %d; Action: %s", vm.Id, action))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.Successf("vm_action_success", action))
	}
}

func panelVMVnc(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}
	url, err := vm.Vnc()
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		RenderTemplate(w, "panel", "vnc", url)
	}
}

type VMReimageForm struct {
	Image int `schema:"image"`
}
func panelVMReimage(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormattedError("invalid_vm"))
		return
	}

	form := new(VMReimageForm)
	err = decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/panel/vm/%d", vmId), 303)
		return
	}

	err = vmReimage(db, session.UserId, int(vmId), form.Image)
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vmId), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Re-image VM", fmt.Sprintf("VM ID: %d; Image: %d", vmId, form.Image))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vmId), L.Success("vm_reimaging"))
	}
}

func panelVMSnapshot(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}

	_, err = vm.Snapshot(r.PostFormValue("name"))
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Snapshot", fmt.Sprintf("VM ID: %d; Name: %s", vm.Id, r.PostFormValue("name")))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.Success("snapshot_creating"))
	}
}

type VMResizeForm struct {
	PlanId int `schema:"plan_id"`
}
func panelVMResize(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}

	form := new(VMResizeForm)
	err = decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), 303)
		return
	}

	err = vm.Resize(form.PlanId)
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Resize", fmt.Sprintf("VM ID: %d; New Plan: %s", vm.Id, form.PlanId))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.Success("vm_resized"))
	}
}

func panelVMRename(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	vm, err := panelVMProcess(w, r, db, session, frameParams)
	if err != nil {
		redirectMessage(w, r, "/panel/vms", L.FormatError(err))
	}
	err = vm.Rename(r.PostFormValue("name"))
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Rename VM", fmt.Sprintf("VM ID: %d; Name: %d", vm.Id, r.PostFormValue("name")))
		redirectMessage(w, r, fmt.Sprintf("/panel/vm/%d", vm.Id), L.Success("vm_renamed"))
	}
}

type PanelBillingParams struct {
	Frame FrameParams
	CreditSummary *CreditSummary
	PaymentMethods []string
}
func panelBilling(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := PanelBillingParams{}
	params.Frame = frameParams
	params.CreditSummary = userCreditSummary(db, session.UserId)
	params.PaymentMethods = paymentMethodList()
	RenderTemplate(w, "panel", "billing", params)
}

type PayForm struct {
	Gateway string `schema:"gateway"`
	Amount float64 `schema:"amount"`
}
func panelPay(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	form := new(PayForm)
	err := decoder.Decode(form, r.Form)
	if err != nil {
		http.Redirect(w, r, "/panel/billing", 303)
		return
	}

	user := userDetails(db, session.UserId)
	paymentHandle(form.Gateway, w, r, db, frameParams, session.UserId, user.Username, form.Amount)
}

type PanelChargesParams struct {
	Frame FrameParams
	Year int
	Month time.Month
	Charges []*Charge

	Previous time.Time
	Next time.Time
}
func panelCharges(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	year, err := strconv.ParseInt(mux.Vars(r)["year"], 10, 32)
	if err != nil {
		year = int64(time.Now().Year())
	}
	month, err := strconv.ParseInt(mux.Vars(r)["month"], 10, 32)
	if err != nil {
		month = int64(time.Now().Month())
	}

	requestTime := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	params := PanelChargesParams{}
	params.Frame = frameParams
	params.Year = int(year)
	params.Month = time.Month(month)
	params.Charges = chargeList(db, session.UserId, params.Year, params.Month)
	params.Previous = requestTime.AddDate(0, -1, 0)
	params.Next = requestTime.AddDate(0, 1, 0)
	RenderTemplate(w, "panel", "charges", params)
}

type PanelAccountParams struct {
	Frame FrameParams
	User *User
	Keys []*ApiKey
	Token string
}
func panelAccount(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := PanelAccountParams{}
	params.Frame = frameParams
	params.User = userDetails(db, session.UserId)
	params.Keys = apiList(db, session.UserId)
	params.Token = csrfGenerate(db, session)
	RenderTemplate(w, "panel", "account", params)
}

type AccountPasswordForm struct {
	OldPassword string `schema:"old_password"`
	NewPassword string `schema:"new_password"`
	NewPasswordConfirm string `schema:"new_password_confirm"`
}
func panelAccountPassword(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	form := new(AccountPasswordForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/panel/account", 303)
		return
	} else if form.NewPassword != form.NewPasswordConfirm {
		redirectMessage(w, r, "/panel/account", L.FormattedError("new_password_mismatch"))
	}

	err = authChangePassword(db, extractIP(r.RemoteAddr), session.UserId, form.OldPassword, form.NewPassword)
	if err != nil {
		redirectMessage(w, r, "/panel/account", L.FormatError(err))
	} else {
		redirectMessage(w, r, "/panel/account", L.Success("password_changed"))
	}
}

type ApiAddForm struct {
	Label string `schema:"label"`
	RestrictAction string `schema:"restrict_action"`
	RestrictIp string `schema:"restrict_ip"`
}
func panelApiAdd(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	form := new(ApiAddForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/panel/account", 303)
		return
	}

	key, err := apiCreate(db, session.UserId, form.Label, form.RestrictAction, form.RestrictIp)
	if err != nil {
		redirectMessage(w, r, "/panel/account", L.FormatError(err))
	} else {
		redirectMessage(w, r, "/panel/account", L.Successf("api_added", key.ApiId, key.ApiKey))
	}
}

func panelApiRemove(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/account", L.FormattedError("invalid_id"))
		return
	}
	apiDelete(db, session.UserId, int(id))
	redirectMessage(w, r, "/panel/account", L.Success("api_deleted"))
}

type PanelImagesParams struct {
	Frame FrameParams
	Images []*Image
	Regions []string
	Token string
}
func panelImages(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := PanelImagesParams{}
	params.Frame = frameParams
	params.Regions = regionList()
	params.Token = csrfGenerate(db, session)

	for _, image := range imageList(db, session.UserId) {
		if image.UserId == session.UserId {
			params.Images = append(params.Images, image)
		}
	}

	RenderTemplate(w, "panel", "images", params)
}

type ImageAddForm struct {
	Region string `schema:"region"`
	Name string `schema:"name"`
	Location string `schema:"location"`
	Format string `schema:"format"`
}
func panelImageAdd(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	form := new(ImageAddForm)
	err := decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, "/panel/images", 303)
		return
	}

	_, err = imageFetch(db, session.UserId, form.Region, form.Name, form.Location, form.Format)
	if err != nil {
		redirectMessage(w, r, "/panel/images", L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Add image", fmt.Sprintf("Location: %s; Format: %s; Name: %s", form.Location, form.Format, form.Name))
		redirectMessage(w, r, "/panel/images", L.Success("image_downloading"))
	}
}

func panelImageRemove(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	imageId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/images", L.FormattedError("invalid_image"))
		return
	}

	err = imageDelete(db, session.UserId, int(imageId))
	if err != nil {
		redirectMessage(w, r, "/panel/images", L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Remove image", fmt.Sprintf("ID: %d", imageId))
		redirectMessage(w, r, "/panel/images", L.Success("image_deleted"))
	}
}

type PanelImageDetailsParams struct {
	Frame FrameParams
	Image *Image
}
func panelImageDetails(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	imageId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/images", L.FormattedError("invalid_image"))
		return
	}
	image := imageInfo(db, session.UserId, int(imageId))
	if image == nil {
		redirectMessage(w, r, "/panel/images", L.FormattedError("image_not_found"))
		return
	}

	params := PanelImageDetailsParams{}
	params.Frame = frameParams
	params.Image = image
	RenderTemplate(w, "panel", "image_details", params)
}

type SupportParams struct {
	Frame FrameParams
	Tickets []*Ticket
}
func panelSupport(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	params := SupportParams{}
	params.Frame = frameParams
	params.Tickets = ticketList(db, session.UserId)
	RenderTemplate(w, "panel", "support", params)
}

type SupportOpenForm struct {
	Name string `schema:"name"`
	Message string `schema:"message"`
}
func panelSupportOpen(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	if r.Method == "POST" {
		form := new(SupportOpenForm)
		err := decoder.Decode(form, r.PostForm)
		if err != nil {
			http.Redirect(w, r, "/panel/support/open", 303)
			return
		}

		ticketId, err := ticketOpen(db, session.UserId, form.Name, form.Message, false)
		if err != nil {
			redirectMessage(w, r, "/panel/support/open", L.FormatError(err))
		} else {
			LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Open ticket", fmt.Sprintf("Subject: %s; Ticket ID: %d", form.Name, ticketId))
			http.Redirect(w, r, fmt.Sprintf("/panel/support/%d", ticketId), 303)
		}
		return
	}

	RenderTemplate(w, "panel", "support_open", PanelFormParams{Frame: frameParams, Token: csrfGenerate(db, session)})
}

type PanelSupportTicketParams struct {
	Frame FrameParams
	Ticket *Ticket
	Token string
}
func panelSupportTicket(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/support", L.FormattedError("invalid_ticket"))
		return
	}
	ticket := ticketDetails(db, session.UserId, int(ticketId), false)
	if ticket == nil {
		redirectMessage(w, r, "/panel/support", L.FormattedError("ticket_not_found"))
		return
	}

	params := PanelSupportTicketParams{}
	params.Frame = frameParams
	params.Ticket = ticket
	params.Token = csrfGenerate(db, session)
	RenderTemplate(w, "panel", "support_ticket", params)
}

type SupportTicketReplyForm struct {
	Message string `schema:"message"`
}
func panelSupportTicketReply(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/support", L.FormattedError("invalid_ticket"))
		return
	}
	form := new(SupportTicketReplyForm)
	err = decoder.Decode(form, r.PostForm)
	if err != nil {
		http.Redirect(w, r, fmt.Sprintf("/panel/support/%d", ticketId), 303)
		return
	}

	err = ticketReply(db, session.UserId, int(ticketId), form.Message, false)
	if err != nil {
		redirectMessage(w, r, fmt.Sprintf("/panel/support/%d", ticketId), L.FormatError(err))
	} else {
		LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Ticket reply", fmt.Sprintf("Ticket ID: %d", ticketId))
		http.Redirect(w, r, fmt.Sprintf("/panel/support/%d", ticketId), 303)
	}
}

func panelSupportTicketClose(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	ticketId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		redirectMessage(w, r, "/panel/support", L.FormattedError("invalid_ticket"))
		return
	}
	ticketClose(db, session.UserId, int(ticketId))
	LogAction(db, session.UserId, extractIP(r.RemoteAddr), "Close ticket", fmt.Sprintf("Ticket ID: %d", ticketId))
	redirectMessage(w, r, fmt.Sprintf("/panel/support/%d", ticketId), L.Success("ticket_closed"))
}

func panelToken(w http.ResponseWriter, r *http.Request, db *Database, session *Session, frameParams FrameParams) {
	w.Write([]byte(csrfGenerate(db, session)))
}
