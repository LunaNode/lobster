package lobster

import "errors"
import "fmt"
import "log"
import "sort"
import "strings"
import "time"

// database objects

type VirtualMachine struct {
	Id             int
	UserId         int
	Region         string
	Name           string
	Identification string
	Status         string
	TaskPending    bool
	ExternalIP     string
	PrivateIP      string
	CreatedTime    time.Time
	Suspended      string
	Plan           Plan

	Info      *VmInfo
	Addresses []*IpAddress
}

// interface objects

type VmInfo struct {
	Ip            string
	PrivateIp     string
	Status        string
	Hostname      string
	BandwidthUsed int64 // in bytes
	LoginDetails  string
	Details       map[string]string
	Actions       []*VmActionDescriptor

	// these fields are filled in by lobster, so VM interface should generally not set
	// occassionally it may be useful for interface to override though
	//   these are autodetected from whether we can cast the interface, so if
	//    interface discovers that some capabilities aren't supported on some
	//    virtual machines, it may want to override that
	//   in that event it should set OverrideCapabilities
	CanVnc               bool
	CanReimage           bool
	CanSnapshot          bool
	CanResize            bool
	CanAddresses         bool
	OverrideCapabilities bool
	PendingSnapshots     []*Image
}

type IpAddress struct {
	Ip        string
	PrivateIp string // blank means N/A

	CanRdns  bool
	Hostname string // current rDNS setting, always blank if CanRdns is false
}

// describes an action that we can perform on a virtual machine
type VmActionDescriptor struct {
	Action      string
	Name        string            // used for button text
	Options     map[string]string // if non-nil, set of options to offer in modal / dropdown menu; not used for sanitization!
	Description string            // if non-empty, will be displayed in a modal
	Dangerous   bool              // if true, we will have confirmation window
}

var regionInterfaces map[string]VmInterface = make(map[string]VmInterface)

const VM_QUERY = "SELECT vms.id, vms.user_id, vms.region, vms.name, vms.identification, " +
	"vms.status, vms.task_pending, vms.external_ip, vms.private_ip, " +
	"vms.time_created, vms.suspended, vms.plan_id, plans.name, plans.price, " +
	"plans.ram, plans.cpu, plans.storage, plans.bandwidth " +
	"FROM vms, plans " +
	"WHERE vms.plan_id = plans.id"

func vmListHelper(rows Rows) []*VirtualMachine {
	vms := make([]*VirtualMachine, 0)
	for rows.Next() {
		var vm VirtualMachine
		rows.Scan(
			&vm.Id,
			&vm.UserId,
			&vm.Region,
			&vm.Name,
			&vm.Identification,
			&vm.Status,
			&vm.TaskPending,
			&vm.ExternalIP,
			&vm.PrivateIP,
			&vm.CreatedTime,
			&vm.Suspended,
			&vm.Plan.Id,
			&vm.Plan.Name,
			&vm.Plan.Price,
			&vm.Plan.Ram,
			&vm.Plan.Cpu,
			&vm.Plan.Storage,
			&vm.Plan.Bandwidth,
		)
		vms = append(vms, &vm)
	}
	return vms
}

func vmList(userId int) []*VirtualMachine {
	return vmListHelper(db.Query(VM_QUERY+" AND vms.user_id = ? ORDER BY id DESC", userId))
}

func vmListRegion(userId int, region string) []*VirtualMachine {
	return vmListHelper(db.Query(VM_QUERY+" AND vms.user_id = ? AND region = ? ORDER BY id DESC", userId, region))
}

func vmGet(vmId int) *VirtualMachine {
	vms := vmListHelper(db.Query(VM_QUERY+" AND vms.id = ? ORDER BY id DESC", vmId))
	if len(vms) == 1 {
		return vms[0]
	} else {
		return nil
	}
}

func vmGetUser(userId int, vmId int) *VirtualMachine {
	vms := vmListHelper(db.Query(VM_QUERY+" AND vms.id = ? AND vms.user_id = ? ORDER BY id DESC", vmId, userId))
	if len(vms) == 1 {
		return vms[0]
	} else {
		return nil
	}
}

func vmGetInterface(region string) VmInterface {
	vmi, ok := regionInterfaces[region]
	if !ok {
		panic(errors.New("no interface registered for " + region))
	}
	return vmi
}

func regionList() []string {
	var regions []string
	for region := range regionInterfaces {
		regions = append(regions, region)
	}
	sort.Sort(sort.StringSlice(regions))
	return regions
}

func vmNameOk(name string) error {
	if len(name) == 0 {
		return L.Error("name_empty")
	} else if len(name) > MAX_VM_NAME_LENGTH {
		return L.Errorf("name_too_long", MAX_VM_NAME_LENGTH)
	} else if !isPrintable(name) {
		return L.Error("invalid_name_format")
	} else {
		return nil
	}
}

func vmCreate(userId int, name string, planId int, imageId int) (int, error) {
	// validate credit
	user := UserDetails(userId)
	if user == nil {
		return 0, L.Error("invalid_account")
	} else if user.Credit < MINIMUM_CREDIT {
		return 0, L.Error("insufficient_credit")
	}

	// validate limit
	var vmCount int
	db.QueryRow("SELECT COUNT(*) FROM vms WHERE user_id = ?", userId).Scan(&vmCount)
	if vmCount >= user.VmLimit {
		return 0, L.Error("exceeded_vm_limit")
	}

	// validate name
	err := vmNameOk(name)
	if err != nil {
		return 0, err
	}

	// validate image ID
	// the image also determines which region we provision in
	image := imageGet(userId, imageId)
	if image == nil {
		return 0, L.Error("image_not_exist")
	} else if image.Status != "active" {
		return 0, L.Error("image_not_ready")
	}

	// validate plan
	plan := planGetRegion(image.Region, planId)
	if plan == nil {
		return 0, L.Error("no_such_plan")
	}

	// create the virtual machine asynchronously
	result := db.Exec("INSERT INTO vms (user_id, region, plan_id, name, status) VALUES (?, ?, ?, ?, ?)", userId, image.Region, planId, name, "provisioning")
	vmId := result.LastInsertId()

	go func() {
		defer errorHandler(nil, nil, true)
		vm := vmGet(vmId)
		vm.Plan = *plan // use plan from planGetRegion so that we have the region-specific identification
		vmIdentification, err := vmGetInterface(image.Region).VmCreate(vm, image.Identification)
		if err != nil {
			ReportError(
				err,
				"vm creation failed",
				fmt.Sprintf("hostname=%s, plan_id=%d, image_identification=%s", name, plan.Id, image.Identification),
			)
			db.Query("UPDATE vms SET status = 'error' WHERE id = ?", vmId)
			MailWrap(userId, "vmCreateError", VmCreateErrorEmail{Id: vmId, Name: name}, true)
			return
		}

		db.Exec("UPDATE vms SET status = 'active', identification = ? WHERE id = ?", vmIdentification, vmId)
		MailWrap(userId, "vmCreate", VmCreateEmail{Id: vmId, Name: name}, true)
	}()

	return vmId, nil
}

func (vm *VirtualMachine) LoadInfo() {
	if vm.Info != nil {
		return
	}

	if vm.Identification == "" || vm.Status != "active" {
		vm.Info = &VmInfo{Ip: L.T("pending"), PrivateIp: L.T("pending"), Status: strings.Title(vm.Status), Hostname: vm.Name}
		return
	}

	vmi := vmGetInterface(vm.Region)

	var err error
	vm.Info, err = vmi.VmInfo(vm)
	if err != nil {
		ReportError(err, "vmInfo failed", fmt.Sprintf("vm_id=%d, identification=%s", vm.Id, vm.Identification))
		vm.Info = new(VmInfo)
	}

	if vm.Info.Hostname == "" {
		vm.Info.Hostname = vm.Name
	}
	if vm.Info.Ip == "" {
		vm.Info.Ip = L.T("pending")

		if vm.Info.PrivateIp == "" {
			vm.Info.PrivateIp = L.T("pending")
		}
	} else {
		db.Exec("UPDATE vms SET external_ip = ?, private_ip = ? WHERE id = ?", vm.Info.Ip, vm.Info.PrivateIp, vm.Id)
	}
	if vm.Info.Status == "" {
		vm.Info.Status = L.T("unknown")
	}

	if !vm.Info.OverrideCapabilities {
		_, vm.Info.CanVnc = vmi.(VMIVnc)
		_, vm.Info.CanReimage = vmi.(VMIReimage)
		_, vm.Info.CanSnapshot = vmi.(VMISnapshot)
		_, vm.Info.CanResize = vmi.(VMIResize)
		_, vm.Info.CanAddresses = vmi.(VMIAddresses)
	}

	vm.Info.PendingSnapshots = imageListVmPending(vm.Id)
}

// Attempt to apply function on the provided VM.
func (vm *VirtualMachine) doForce(f func(vm *VirtualMachine) error, ignoreSuspend bool) error {
	if vm.Identification == "" || vm.Status != "active" {
		return L.Error("vm_not_ready")
	} else if vm.Suspended != "no" && !ignoreSuspend {
		if vm.Suspended == "auto" {
			return L.Error("vm_suspended_auto")
		} else if vm.Suspended == "manual" {
			return L.Error("vm_suspended_manual")
		} else {
			return L.Error("vm_suspended")
		}
	} else if vm.TaskPending {
		return L.Error("vm_has_pending_task")
	}

	return f(vm)
}

func (vm *VirtualMachine) do(f func(vm *VirtualMachine) error) error {
	return vm.doForce(f, false)
}

func (vm *VirtualMachine) Start() error {
	log.Printf("vmStart(%d)", vm.Id)
	return vm.do(vmGetInterface(vm.Region).VmStart)
}

func (vm *VirtualMachine) Stop() error {
	log.Printf("vmStop(%d)", vm.Id)
	return vm.doForce(vmGetInterface(vm.Region).VmStop, true)
}

func (vm *VirtualMachine) Reboot() error {
	log.Printf("vmReboot(%d)", vm.Id)
	return vm.do(vmGetInterface(vm.Region).VmReboot)
}

func (vm *VirtualMachine) Action(action string, value string) error {
	log.Printf("vmAction(%d, %s, %s)", vm.Id, action, value)
	return vm.do(func(vm *VirtualMachine) error {
		return vmGetInterface(vm.Region).VmAction(vm, action, value)
	})
}

func (vm *VirtualMachine) Vnc() (string, error) {
	log.Printf("vmVnc(%d)", vm.Id)
	var url string
	err := vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIVnc)
		if !ok {
			return L.Error("vm_vnc_unsupported")
		}

		urlTry, err := vmi.VmVnc(vm)
		if err != nil {
			ReportError(err, "failed to retrieve VNC URL", fmt.Sprintf("vm_id=%d, vm_identification=%s", vm.Id, vm.Identification))
			return err
		} else {
			url = urlTry
			return nil
		}
	})
	return url, err
}

func vmReimage(userId int, vmId int, imageId int) error {
	// validate image ID
	image := imageGet(userId, imageId)
	if image == nil {
		return L.Error("image_not_exist")
	} else if image.Status != "active" {
		return L.Error("image_not_ready")
	}

	vm := vmGetUser(userId, vmId)
	if vm == nil {
		return L.Error("invalid_vm")
	}

	log.Printf("vmReimage(%d, %d, %d)", userId, vmId, imageId)
	return vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIReimage)
		if ok {
			return vmi.VmReimage(vm, image.Identification)
		} else {
			return L.Error("vm_reimage_unsupported")
		}
	})
}

func (vm *VirtualMachine) Snapshot(name string) (int, error) {
	if name == "" {
		return 0, L.Error("name_empty")
	}

	log.Printf("vmSnapshot(%d, %s)", vm.Id, name)
	var imageId int
	err := vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMISnapshot)
		if ok {
			imageIdentification, err := vmi.VmSnapshot(vm)
			if err != nil {
				return err
			}
			result := db.Exec(
				"INSERT INTO images (user_id, region, name, identification, status, source_vm) "+
					"VALUES (?, ?, ?, ?, 'pending', ?)",
				vm.UserId, vm.Region, name, imageIdentification, vm.Id,
			)
			imageId = result.LastInsertId()
			return nil
		} else {
			return L.Error("vm_snapshot_unsupported")
		}
	})
	return imageId, err
}

func (vm *VirtualMachine) Resize(planId int) error {
	plan := planGetRegion(vm.Region, planId)
	if plan == nil {
		return L.Error("no_such_plan")
	}

	log.Printf("vmResize(%d, %d)", vm.Id, planId)
	return vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIResize)
		if ok {
			err := vmi.VmResize(vm, plan)
			if err != nil {
				return err
			}

			// we need to be careful in our bandwidth accounting across resizes; the user should get both:
			//   a) old plan's allocation for the time provisioned so far this month
			//   b) new plan's allocation for the remainder of the month
			// we handle this by treating it as a deletion and re-creation of the VM, i.e. call vmUpdateAdditionalBandwidth and reset VM creation time
			vmUpdateAdditionalBandwidth(vm)
			db.Exec("UPDATE vms SET plan_id = ?, time_created = NOW() WHERE id = ?", plan.Id, vm.Id)
			return nil
		} else {
			return L.Error("vm_resize_unsupported")
		}
	})
}

func (vm *VirtualMachine) Rename(name string) error {
	// validate name
	err := vmNameOk(name)
	if err != nil {
		return err
	}

	log.Printf("vmRename(%d, %s)", vm.Id, name)

	return vm.do(func(vm *VirtualMachine) error {
		db.Exec("UPDATE vms SET name = ? WHERE id = ?", name, vm.Id)

		// don't worry about back-end errors, but try to rename anyway
		vmi, ok := vmGetInterface(vm.Region).(VMIRename)
		if ok {
			ReportError(
				vmi.VmRename(vm, name),
				"VM rename failed",
				fmt.Sprintf("id: %d, identification: %d, name: %s", vm.Id, vm.Identification, name),
			)
		}
		return nil
	})
}

func (vm *VirtualMachine) LoadAddresses() error {
	if vm.Addresses != nil {
		return nil
	} else if vm.Identification == "" || vm.Status != "active" {
		return L.Error("vm_not_ready")
	}

	vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
	if !ok {
		return L.Error("operation_unsupported")
	}
	var err error
	vm.Addresses, err = vmi.VmAddresses(vm)
	return err
}

func (vm *VirtualMachine) AddAddress() error {
	err := vm.LoadAddresses()
	if err != nil {
		return err
	} else if len(vm.Addresses) >= cfg.Vm.MaximumIps {
		return L.Errorf("vm_max_ips", cfg.Vm.MaximumIps)
	} else if cfg.Vm.MaximumIps <= 0 {
		return L.Error("ip_manage_disabled")
	}

	vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
	if !ok {
		return L.Error("operation_unsupported")
	}
	return vm.do(vmi.VmAddAddress)
}

func (vm *VirtualMachine) RemoveAddress(ip string, privateip string) error {
	return vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
		if ok {
			return vmi.VmRemoveAddress(vm, ip, privateip)
		} else {
			return L.Error("operation_unsupported")
		}
	})
}

func (vm *VirtualMachine) SetRdns(ip string, hostname string) error {
	return vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
		if ok {
			return vmi.VmSetRdns(vm, ip, hostname)
		} else {
			return L.Error("operation_unsupported")
		}
	})
}

func (vm *VirtualMachine) Delete(userId int) error {
	if vm.UserId != userId {
		return L.Error("invalid_vm")
	} else if vm.Status == "provisioning" {
		return L.Error("vm_not_ready")
	}

	log.Printf("vmDelete(%d, %d)", userId, vm.Id)

	if vm.Identification != "" {
		go func() {
			ReportError(
				vmGetInterface(vm.Region).VmDelete(vm),
				"failed to delete VM",
				fmt.Sprintf("vm_id=%d, vm_identification=%s", vm.Id, vm.Identification),
			)
		}()
	}

	vmBilling(vm.Id, true)
	vmUpdateAdditionalBandwidth(vm)
	db.Exec("DELETE FROM vms WHERE id = ?", vm.Id)
	MailWrap(userId, "vmDeleted", VmDeletedEmail{Id: vm.Id, Name: vm.Name}, true)
	return nil
}

func (vm *VirtualMachine) Suspend(auto bool) {
	if auto {
		db.Exec("UPDATE vms SET suspended = 'auto' WHERE id = ? AND suspended = 'no'", vm.Id)
	} else {
		db.Exec("UPDATE vms SET suspended = 'manual' WHERE id = ?", vm.Id)
	}

	// Try to stop the VM, and throw an error if it's not stopped after one minute
	// We ignonre error from Stop function since it might throw error if VM already stopped
	go func() {
		defer errorHandler(nil, nil, true)
		vm.Stop()
		time.Sleep(time.Minute)
		info, err := vmGetInterface(vm.Region).VmInfo(vm)
		if err != nil {
			ReportError(err, "failed to suspend VM", fmt.Sprintf("user_id: %d, vm_id: %d", vm.UserId, vm.Id))
		} else if info.Status != "Offline" {
			ReportError(errors.New("status not offline after one minute"), "failed to suspend VM", fmt.Sprintf("user_id: %d, vm_id: %d", vm.UserId, vm.Id))
		}
	}()
}

func (vm *VirtualMachine) Unsuspend() error {
	db.Exec("UPDATE vms SET suspended = 'no' WHERE id = ?", vm.Id)
	vm.Suspended = "no"
	return vm.Start()
}

func (vm *VirtualMachine) SetMetadata(k string, v string) {
	rows := db.Query("SELECT id FROM vm_metadata WHERE vm_id = ? AND k = ?", vm.Id, k)
	if rows.Next() {
		var rowId int
		rows.Scan(&rowId)
		rows.Close()
		db.Exec("UPDATE vm_metadata SET v = ? WHERE id = ?", v, rowId)
	} else {
		db.Exec("INSERT INTO vm_metadata (vm_id, k, v) VALUES (?, ?, ?)", vm.Id, k, v)
	}
}

// Returns the metadata value if set, or d (default) otherwise.
func (vm *VirtualMachine) Metadata(k string, d string) string {
	rows := db.Query("SELECT v FROM vm_metadata WHERE vm_id = ? AND k = ?", vm.Id, k)
	if rows.Next() {
		var v string
		rows.Scan(&v)
		rows.Close()
		return v
	} else {
		return d
	}
}

// vmUpdateAdditionalBandwidth is called on VM deletion or resize, to add the VM's bandwidth to user's bandwidth pool
func vmUpdateAdditionalBandwidth(vm *VirtualMachine) {
	// determine how much of the plan bandwidth to add to the user's bandwidth pool for current month
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)
	var factor float64

	if vm.CreatedTime.Before(monthStart) {
		factor = float64(now.Sub(monthStart))
	} else {
		factor = float64(now.Sub(vm.CreatedTime))
	}

	factor /= float64(monthEnd.Sub(monthStart))
	if factor > 1 {
		factor = 1
	}

	additionalBandwidth := int64((factor*float64(vm.Plan.Bandwidth) + 15) * 1024 * 1024 * 1024)
	rows := db.Query("SELECT id FROM region_bandwidth WHERE region = ? AND user_id = ?", vm.Region, vm.UserId)
	if rows.Next() {
		var rowId int
		rows.Scan(&rowId)
		rows.Close()
		db.Exec(
			"UPDATE region_bandwidth "+
				"SET bandwidth_additional = bandwidth_additional + ? "+
				"WHERE id = ?",
			additionalBandwidth, rowId,
		)
	} else {
		db.Exec(
			"INSERT INTO region_bandwidth (user_id, region, bandwidth_additional) "+
				"VALUES (?, ?, ?)",
			vm.UserId, vm.Region, additionalBandwidth,
		)
	}
}

// Attempts to bill on the specified virtual machine.
// terminating should be set to true if the VM is about to be deleted, so that we:
//  a) bill for the last used interval
//  b) enforce BILLING_VM_MINIMUM
func vmBilling(vmId int, terminating bool) {
	db.Exec("UPDATE vms SET time_billed = time_created WHERE time_billed = 0")
	rows := db.Query("SELECT TIMESTAMPDIFF(MINUTE, time_billed, NOW()) FROM vms WHERE id = ?", vmId)

	if !rows.Next() {
		log.Printf("Warning: vmBilling called with non-existent virtual machine id=%d", vmId)
		return
	}

	var minutes int
	rows.Scan(&minutes)
	rows.Close()
	intervals := minutes / cfg.Billing.BillingInterval
	if terminating {
		intervals++

		// enforce minimum billing intervals if needed
		if cfg.Billing.BillingVmMinimum > 1 {
			rows = db.Query("SELECT TIMESTAMPDIFF(MINUTE, time_created, time_billed) FROM vms WHERE id = ?", vmId)
			if rows.Next() {
				var alreadyBilledMinutes int
				rows.Scan(&alreadyBilledMinutes)
				rows.Close()
				alreadyBilledIntervals := alreadyBilledMinutes / cfg.Billing.BillingInterval
				if alreadyBilledIntervals+intervals < cfg.Billing.BillingVmMinimum {
					intervals = cfg.Billing.BillingVmMinimum - alreadyBilledIntervals
				}
			}
		}
	}
	if intervals == 0 {
		return
	}

	vm := vmGet(vmId)
	if vm == nil {
		log.Printf("vmBilling: vm id=%d disappeared during execution", vmId)
		return
	} else if vm.Status != "active" {
		return
	}

	amount := int64(intervals) * int64(vm.Plan.Price)
	UserApplyCharge(vm.UserId, vm.Name, "Plan: "+vm.Plan.Name, fmt.Sprintf("vm-%d", vmId), amount)
	db.Exec("UPDATE vms SET time_billed = DATE_ADD(time_billed, INTERVAL ? MINUTE) WHERE id = ?", intervals*cfg.Billing.BillingInterval, vmId)

	// also bill for bandwidth usage
	newBytesUsed := vmGetInterface(vm.Region).BandwidthAccounting(vm)
	if newBytesUsed > 0 {
		rows := db.Query("SELECT id FROM region_bandwidth WHERE user_id = ? AND region = ?", vm.UserId, vm.Region)

		if rows.Next() {
			var rowId int
			rows.Scan(&rowId)
			rows.Close()
			db.Exec("UPDATE region_bandwidth SET bandwidth_used = bandwidth_used + ? WHERE id = ?", newBytesUsed, rowId)
		} else {
			db.Exec(
				"INSERT INTO region_bandwidth (user_id, region, bandwidth_used) "+
					"VALUES (?, ?, ?)",
				vm.UserId, vm.Region, newBytesUsed,
			)
		}
	}
}

// Bills for used storage space and other resources hourly.
func serviceBilling() {
	db.Exec("UPDATE users SET time_billed = NOW() WHERE time_billed = 0")
	rows := db.Query(
		"SELECT id, TIMESTAMPDIFF(HOUR, time_billed, NOW()) " +
			"FROM users " +
			"WHERE status = 'active' AND TIMESTAMPDIFF(HOUR, time_billed, NOW()) > 0",
	)
	defer rows.Close()

	for rows.Next() {
		var userId, hours int
		rows.Scan(&userId, &hours)
		var hourlyCharge int64 = 0

		// bill storage space
		var storageBytes int64 = 0
		for _, image := range imageList(userId) {
			if image.UserId == userId {
				details := imageInfo(userId, image.Id)
				if details != nil && details.Info.Size > 0 {
					storageBytes += details.Info.Size
				}
			}
		}
		creditPerGBHour := int64(cfg.Billing.StorageFee * BILLING_PRECISION)
		hourlyCharge += storageBytes * creditPerGBHour / 1000 / 1000 / 1000

		if hourlyCharge > 0 {
			totalCharge := hourlyCharge * int64(hours)
			log.Printf("Charging user %d for %d bytes (amount=%.5f)", userId, storageBytes, float64(totalCharge)/BILLING_PRECISION)
			UserApplyCharge(userId, "Image storage space", fmt.Sprintf("%d MB", storageBytes/1000/1000), "storage", totalCharge)
		}

		db.Exec("UPDATE users SET time_billed = DATE_ADD(time_billed, INTERVAL ? HOUR) WHERE id = ?", hours, userId)
	}
}
