package lobster

import "database/sql"
import "errors"
import "fmt"
import "log"
import "strings"
import "time"

// database objects

type VirtualMachine struct {
	Id int
	UserId int
	Region string
	Name string
	Identification string
	Status string
	TaskPending bool
	ExternalIP string
	PrivateIP string
	CreatedTime time.Time
	Suspended string
	Plan Plan

	Info *VmInfo
	Addresses []*IpAddress
	db *Database
}

type Image struct {
	Id int
	UserId int
	Region string
	Name string
	Identification string
	Status string

	Info *ImageInfo
}

type Plan struct {
	Id int
	Name string
	Price int64
	Ram int
	Cpu int
	Storage int
	Bandwidth int
}

// interface objects

type VmInfo struct {
	Ip string
	PrivateIp string
	Status string
	Hostname string
	BandwidthUsed int64 // in bytes
	LoginDetails string
	Details map[string]string
	Actions []*VmActionDescriptor

	// these fields are filled in by lobster, so VM interface should generally not set
	// occassionally it may be useful for interface to override though
	//   these are autodetected from whether we can cast the interface, so if
	//    interface discovers that some capabilities aren't supported on some
	//    virtual machines, it may want to override that
	//   in that event it should set OverrideCapabilities
	CanVnc bool
	CanReimage bool
	CanSnapshot bool
	CanAddresses bool
	OverrideCapabilities bool
}

type IpAddress struct {
	Ip string
	PrivateIp string // blank means N/A

	CanRdns bool
	Hostname string // current rDNS setting, always blank if CanRdns is false
}

type ImageStatus int
const (
	ImagePending ImageStatus = iota
	ImageActive
	ImageError
)

type ImageInfo struct {
	Size int64
	Status ImageStatus
	Details map[string]string
}

// describes an action that we can perform on a virtual machine
type VmActionDescriptor struct {
	Action string
	Name string // used for button text
	Options map[string]string // if non-nil, set of options to offer in modal / dropdown menu; not used for sanitization!
	Description string // if non-empty, will be displayed in a modal
	Dangerous bool // if true, we will have confirmation window
}

var regionInterfaces map[string]VmInterface = make(map[string]VmInterface)

const VM_QUERY = "SELECT vms.id, vms.user_id, vms.region, vms.name, vms.identification, vms.status, vms.task_pending, vms.external_ip, vms.private_ip, vms.time_created, vms.suspended, vms.plan_id, plans.name, plans.price, plans.ram, plans.cpu, plans.storage, plans.bandwidth FROM vms, plans WHERE vms.plan_id = plans.id"

func vmListHelper(db *Database, rows *sql.Rows) []*VirtualMachine {
	vms := make([]*VirtualMachine, 0)
	for rows.Next() {
		vm := VirtualMachine{db: db}
		rows.Scan(&vm.Id, &vm.UserId, &vm.Region, &vm.Name, &vm.Identification, &vm.Status, &vm.TaskPending, &vm.ExternalIP, &vm.PrivateIP, &vm.CreatedTime, &vm.Suspended, &vm.Plan.Id, &vm.Plan.Name, &vm.Plan.Price, &vm.Plan.Ram, &vm.Plan.Cpu, &vm.Plan.Storage, &vm.Plan.Bandwidth)
		vms = append(vms, &vm)
	}
	return vms
}

func vmList(db *Database, userId int) []*VirtualMachine {
	return vmListHelper(db, db.Query(VM_QUERY + " AND vms.user_id = ? ORDER BY id DESC", userId))
}

func vmListRegion(db *Database, userId int, region string) []*VirtualMachine {
	return vmListHelper(db, db.Query(VM_QUERY + " AND vms.user_id = ? AND region = ? ORDER BY id DESC", userId, region))
}

func vmGet(db *Database, vmId int) *VirtualMachine {
	vms := vmListHelper(db, db.Query(VM_QUERY + " AND vms.id = ? ORDER BY id DESC", vmId))
	if len(vms) == 1 {
		return vms[0]
	} else {
		return nil
	}
}

func vmGetUser(db *Database, userId int, vmId int) *VirtualMachine {
	vms := vmListHelper(db, db.Query(VM_QUERY + " AND vms.id = ? AND vms.user_id = ? ORDER BY id DESC", vmId, userId))
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
	for region, _ := range regionInterfaces {
		regions = append(regions, region)
	}
	return regions
}

func vmNameOk(name string) error {
	if len(name) == 0 {
		return errors.New("name cannot be empty")
	} else if len(name) > MAX_VM_NAME_LENGTH {
		return errors.New(fmt.Sprintf("name cannot exceed %d characters", MAX_VM_NAME_LENGTH))
	} else if !isPrintable(name) {
		return errors.New("provided name contains invalid characters")
	} else {
		return nil
	}
}

func vmCreate(db *Database, userId int, name string, planId int, imageId int) (int, error) {
	// validate credit
	user := userDetails(db, userId)
	if user == nil {
		return 0, errors.New("invalid user account")
	} else if user.Credit < MINIMUM_CREDIT {
		return 0, errors.New("insufficient credit (make a payment from the Billing tab)")
	}

	// validate limit
	var vmCount int
	db.QueryRow("SELECT COUNT(*) FROM vms WHERE user_id = ?", userId).Scan(&vmCount)
	if vmCount >= user.VmLimit {
		return 0, errors.New("you have exceeded your current VM count limit, please contact support to have your limit increased")
	}

	// validate name
	err := vmNameOk(name)
	if err != nil {
		return 0, err
	}

	// validate image ID
	image := imageGet(db, userId, imageId)
	if image == nil {
		return 0, errors.New("specified image does not exist")
	} else if image.Status != "active" {
		return 0, errors.New("specified image is not ready")
	}

	// validate plan
	plan := planGet(db, planId)
	if plan == nil {
		return 0, errors.New("no such plan")
	}

	// create the virtual machine asynchronously
	result := db.Exec("INSERT INTO vms (user_id, region, plan_id, name, status) VALUES (?, ?, ?, ?, ?)", userId, image.Region, planId, name, "provisioning")
	vmId, err := result.LastInsertId()
	checkErr(err)

	go func() {
		defer errorHandler(nil, nil, true)
		vmIdentification, err := vmGetInterface(image.Region).VmCreate(vmGet(db, int(vmId)), image.Identification)
		if err != nil {
			reportError(err, "vm creation failed", fmt.Sprintf("hostname=%s, plan_id=%d, image_identification=%s", name, plan.Id, image.Identification))
			db.Query("UPDATE vms SET status = 'error' WHERE id = ?", vmId)
			mailWrap(db, userId, "vmCreateError", VmCreateErrorEmail{Id: int(vmId), Name: name}, true)
			return
		}

		db.Exec("UPDATE vms SET status = 'active', identification = ? WHERE id = ?", vmIdentification, vmId)
		mailWrap(db, userId, "vmCreate", VmCreateEmail{Id: int(vmId), Name: name}, true)
	}()

	return int(vmId), nil
}

func (vm *VirtualMachine) LoadInfo() {
	if vm.Info != nil {
		return
	}

	if vm.Identification == "" || vm.Status != "active" {
		vm.Info = &VmInfo{Ip: "Pending", PrivateIp: "Pending", Status: strings.Title(vm.Status), Hostname: vm.Name}
		return
	}

	vmi := vmGetInterface(vm.Region)

	var err error
	vm.Info, err = vmi.VmInfo(vm)
	if err != nil {
		reportError(err, "vmInfo failed", fmt.Sprintf("vm_id=%d, identification=%s", vm.Id, vm.Identification))
		vm.Info = new(VmInfo)
	}

	if vm.Info.Hostname == "" {
		vm.Info.Hostname = vm.Name
	}
	if vm.Info.Ip == "" {
		vm.Info.Ip = "Pending"

		if vm.Info.PrivateIp == "" {
			vm.Info.PrivateIp = "Pending"
		}
	} else {
		vm.db.Exec("UPDATE vms SET external_ip = ?, private_ip = ? WHERE id = ?", vm.Info.Ip, vm.Info.PrivateIp, vm.Id)
	}
	if vm.Info.Status == "" {
		vm.Info.Status = "Unknown"
	}

	if !vm.Info.OverrideCapabilities {
		_, vm.Info.CanVnc = vmi.(VMIVnc)
		_, vm.Info.CanReimage = vmi.(VMIReimage)
		_, vm.Info.CanSnapshot = vmi.(VMISnapshot)
		_, vm.Info.CanAddresses = vmi.(VMIAddresses)
	}
}

// Attempt to apply function on the provided VM.
func (vm *VirtualMachine) do(f func(vm *VirtualMachine) error) error {
	if vm.Identification == "" || vm.Status != "active" {
		return errors.New("VM is not ready yet")
	} else if vm.Suspended != "no" {
		if vm.Suspended == "auto" {
			return errors.New("VM is suspended due to negative credit, make a payment first")
		} else if vm.Suspended == "manual" {
			return errors.New("VM is suspended, please see abuse ticket under Support tab")
		} else {
			return errors.New("VM is suspended")
		}
	} else if vm.TaskPending {
		return errors.New("VM has pending task, please try again later")
	}

	return f(vm)
}
func (vm *VirtualMachine) Start() error {
	log.Printf("vmStart(%d)", vm.Id)
	return vm.do(vmGetInterface(vm.Region).VmStart)
}
func (vm *VirtualMachine) Stop() error {
	log.Printf("vmStop(%d)", vm.Id)
	return vm.do(vmGetInterface(vm.Region).VmStop)
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
			return errors.New("VNC not supported on this VM")
		}

		urlTry, err := vmi.VmVnc(vm)
		if err != nil {
			reportError(err, "failed to retrieve VNC URL", fmt.Sprintf("vm_id=%d, vm_identification=%s", vm.Id, vm.Identification))
			return err
		} else {
			url = urlTry
			return nil
		}
	})
	return url, err
}

func vmReimage(db *Database, userId int, vmId int, imageId int) error {
	// validate image ID
	image := imageGet(db, userId, imageId)
	if image == nil {
		return errors.New("specified image does not exist")
	} else if image.Status != "active" {
		return errors.New("specified image is not ready")
	}

	vm := vmGetUser(db, userId, vmId)
	if vm == nil {
		return errors.New("invalid VM ID")
	}

	log.Printf("vmReimage(%d, %d, %d)", userId, vmId, imageId)
	return vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIReimage)
		if ok {
			return vmi.VmReimage(vm, image.Identification)
		} else {
			return errors.New("re-image is not supported on this VM")
		}
	})
}

func (vm *VirtualMachine) Snapshot(name string) (int, error) {
	if name == "" {
		return 0, errors.New("snapshot name cannot be empty")
	}

	log.Printf("vmSnapshot(%d, %s)", vm.Id, name)
	var imageId int64
	err := vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMISnapshot)
		if ok {
			imageIdentification, err := vmi.VmSnapshot(vm)
			if err != nil {
				return err
			}
			result := vm.db.Exec("INSERT INTO images (user_id, region, name, identification, status) VALUES (?, ?, ?, ?, 'pending')", vm.UserId, vm.Region, name, imageIdentification)
			imageId, _ = result.LastInsertId()
			return nil
		} else {
			return errors.New("snapshot is not supported on this VM")
		}
	})
	return int(imageId), err
}

func (vm *VirtualMachine) Rename(name string) error {
	// validate name
	err := vmNameOk(name)
	if err != nil {
		return err
	}

	log.Printf("vmRename(%d, %s)", vm.Id, name)

	return vm.do(func(vm *VirtualMachine) error {
		vm.db.Exec("UPDATE vms SET name = ? WHERE id = ?", name, vm.Id)

		// don't worry about back-end errors, but try to rename anyway
		vmi, ok := vmGetInterface(vm.Region).(VMIRename)
		if ok {
			reportError(vmi.VmRename(vm, name), "VM rename failed", fmt.Sprintf("id: %d, identification: %d, name: %s", vm.Id, vm.Identification, name))
		}
		return nil
	})
}

func (vm *VirtualMachine) LoadAddresses() error {
	if vm.Addresses != nil {
		return nil
	} else if vm.Identification == "" || vm.Status != "active" {
		return errors.New("VM is not ready yet")
	}

	vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
	if !ok {
		return errors.New("operation not supported")
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
		return errors.New(fmt.Sprintf("this VM already has the maximum of %d IP addresses", cfg.Vm.MaximumIps))
	} else if cfg.Vm.MaximumIps <= 0 {
		return errors.New("IP address management is disabled")
	}

	vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
	if !ok {
		return errors.New("operation not supported")
	}
	return vm.do(vmi.VmAddAddress)
}

func (vm *VirtualMachine) RemoveAddress(ip string, privateip string) error {
	return vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
		if ok {
			return vmi.VmRemoveAddress(vm, ip, privateip)
		} else {
			return errors.New("operation not supported")
		}
	})
}

func (vm *VirtualMachine) SetRdns(ip string, hostname string) error {
	return vm.do(func(vm *VirtualMachine) error {
		vmi, ok := vmGetInterface(vm.Region).(VMIAddresses)
		if ok {
			return vmi.VmSetRdns(vm, ip, hostname)
		} else {
			return errors.New("operation not supported")
		}
	})
}

func (vm *VirtualMachine) Delete(userId int) error {
	if vm.UserId != userId {
		return errors.New("invalid VM instance")
	} else if vm.Status == "provisioning" {
		return errors.New("VM is not ready yet")
	}

	log.Printf("vmDelete(%d, %d)", userId, vm.Id)

	if vm.Identification != "" {
		go func() {
			reportError(vmGetInterface(vm.Region).VmDelete(vm), "failed to delete VM", fmt.Sprintf("vm_id=%d, vm_identification=%s", vm.Id, vm.Identification))
		}()
	}

	vmBilling(vm.db, vm.Id, true)
	vmUpdateAdditionalBandwidth(vm.db, vm)
	vm.db.Exec("DELETE FROM vms WHERE id = ?", vm.Id)
	mailWrap(vm.db, userId, "vmDeleted", VmDeletedEmail{Id: vm.Id, Name: vm.Name}, true)
	return nil
}

func (vm *VirtualMachine) Suspend(auto bool) error {
	err := vm.Stop()
	if err != nil {
		return err
	}
	if auto {
		vm.db.Exec("UPDATE vms SET suspended = 'auto' WHERE id = ? AND suspended = 'no'", vm.Id)
	} else {
		vm.db.Exec("UPDATE vms SET suspended = 'manual' WHERE id = ?", vm.Id)
	}
	return nil
}

func (vm *VirtualMachine) Unsuspend() error {
	vm.db.Exec("UPDATE vms SET suspended = 'no' WHERE id = ?", vm.Id)
	return vm.Start()
}

func (vm *VirtualMachine) SetMetadata(k string, v string) {
	rows := vm.db.Query("SELECT id FROM vm_metadata WHERE vm_id = ? AND k = ?", vm.Id, k)
	if rows.Next() {
		var rowId int
		rows.Scan(&rowId)
		rows.Close()
		vm.db.Exec("UPDATE vm_metadata SET v = ? WHERE id = ?", v, rowId)
	} else {
		vm.db.Exec("INSERT INTO vm_metadata (vm_id, k, v) VALUES (?, ?, ?)", vm.Id, k, v)
	}
}

// Returns the metadata value if set, or d (default) otherwise.
func (vm *VirtualMachine) Metadata(k string, d string) string {
	rows := vm.db.Query("SELECT v FROM vm_metadata WHERE vm_id = ? AND k = ?", vm.Id, k)
	if rows.Next() {
		var v string
		rows.Scan(&v)
		rows.Close()
		return v
	} else {
		return d
	}
}

func planListHelper(rows *sql.Rows) []*Plan {
	defer rows.Close()
	plans := make([]*Plan, 0)
	for rows.Next() {
		plan := Plan{}
		rows.Scan(&plan.Id, &plan.Name, &plan.Price, &plan.Ram, &plan.Cpu, &plan.Storage, &plan.Bandwidth)
		plans = append(plans, &plan)
	}
	return plans
}

func planList(db *Database) []*Plan {
	return planListHelper(db.Query("SELECT id, name, price, ram, cpu, storage, bandwidth FROM plans ORDER BY id"))
}

func planGet(db *Database, planId int) *Plan {
	plans := planListHelper(db.Query("SELECT id, name, price, ram, cpu, storage, bandwidth FROM plans WHERE id = ?", planId))
	if len(plans) == 1 {
		return plans[0]
	} else {
		return nil
	}
}

func planCreate(db *Database, name string, price int64, ram int, cpu int, storage int, bandwidth int) {
	db.Exec("INSERT INTO plans (name, price, ram, cpu, storage, bandwidth) VALUES (?, ?, ?, ?, ?, ?)", name, price, ram, cpu, storage, bandwidth)
}

func planDelete(db *Database, planId int) {
	db.Exec("DELETE FROM plans WHERE id = ?", planId)
}

func imageListHelper(rows *sql.Rows) []*Image {
	defer rows.Close()
	images := make([]*Image, 0)
	for rows.Next() {
		image := Image{}
		rows.Scan(&image.Id, &image.UserId, &image.Region, &image.Name, &image.Identification, &image.Status)
		images = append(images, &image)
	}
	return images
}

func imageListAll(db *Database) []*Image {
	return imageListHelper(db.Query("SELECT id, user_id, region, name, identification, status FROM images ORDER BY user_id, name"))
}

func imageList(db *Database, userId int) []*Image {
	return imageListHelper(db.Query("SELECT id, user_id, region, name, identification, status FROM images WHERE user_id = -1 OR user_id = ? ORDER BY name", userId))
}

func imageListRegion(db *Database, userId int, region string) []*Image {
	return imageListHelper(db.Query("SELECT id, user_id, region, name, identification, status FROM images WHERE (user_id = -1 OR user_id = ?) AND region = ? ORDER BY name", userId, region))
}

func imageGet(db *Database, userId int, imageId int) *Image {
	images := imageListHelper(db.Query("SELECT id, user_id, region, name, identification, status FROM images WHERE id = ? AND (user_id = -1 OR user_id = ?)", imageId, userId))
	if len(images) == 1 {
		return images[0]
	} else {
		return nil
	}
}
func imageGetForce(db *Database, imageId int) *Image {
	images := imageListHelper(db.Query("SELECT id, user_id, region, name, identification, status FROM images WHERE id = ?", imageId))
	if len(images) == 1 {
		return images[0]
	} else {
		return nil
	}
}

func imageFetch(db *Database, userId int, region string, name string, url string, format string) (int, error) {
	// validate credit
	user := userDetails(db, userId)
	if user == nil {
		return 0, errors.New("invalid user account")
	} else if user.Credit < MINIMUM_CREDIT {
		return 0, errors.New("insufficient credit (make a payment from the Billing tab)")
	}

	// validate region
	vmi, ok := regionInterfaces[region]
	if !ok {
		return 0, errors.New("invalid region")
	}

	vmiImage, ok := vmi.(VMIImages)
	if !ok {
		return 0, errors.New("operation not supported")
	}

	imageIdentification, err := vmiImage.ImageFetch(url, format)
	if err != nil {
		return 0, err
	} else {
		result := db.Exec("INSERT INTO images (user_id, region, name, identification, status) VALUES (?, ?, ?, ?, 'pending')", userId, region, name, imageIdentification)
		imageId, _ := result.LastInsertId()
		return int(imageId), nil
	}
}

func imageAdd(db *Database, name string, region string, identification string) {
	db.Exec("INSERT INTO images (name, region, identification) VALUES (?, ?, ?)", name, region, identification)
}

func imageDelete(db *Database, userId int, imageId int) error {
	image := imageGet(db, userId, imageId)
	if image == nil || image.UserId != userId {
		return errors.New("invalid image")
	}

	vmi, ok := vmGetInterface(image.Region).(VMIImages)
	if !ok {
		return errors.New("operation not supported")
	}

	err := vmi.ImageDelete(image.Identification)
	if err != nil {
		return err
	} else {
		db.Exec("DELETE FROM images WHERE id = ?", image.Id)
		return nil
	}
}

func imageDeleteForce(db *Database, imageId int) error {
	image := imageGetForce(db, imageId)
	if image == nil {
		return errors.New("image not found")
	}

	vmi, ok := vmGetInterface(image.Region).(VMIImages)
	if !ok {
		return errors.New("operation not supported")
	}

	err := vmi.ImageDelete(image.Identification)
	if err != nil {
		reportError(err, "image force deletion failed", fmt.Sprintf("image_id=%d, identification=%s", image.Id, image.Identification))
	}
	db.Exec("DELETE FROM images WHERE id = ?", image.Id)
	return nil
}

func imageInfo(db *Database, userId int, imageId int) *Image {
	image := imageGet(db, userId, imageId)
	if image == nil || image.UserId != userId {
		return nil
	}

	vmi, ok := vmGetInterface(image.Region).(VMIImages)
	if !ok {
		return nil
	}

	var err error
	image.Info, err = vmi.ImageInfo(image.Identification)
	if err != nil {
		reportError(err, "imageInfo failed", fmt.Sprintf("image_id=%d, identification=%s", image.Id, image.Identification))
		image.Info = new(ImageInfo)
	}
	return image
}

// vmUpdateAdditionalBandwidth is called on VM deletion, to add the VM's bandwidth to user's bandwidth pool
func vmUpdateAdditionalBandwidth(db *Database, vm *VirtualMachine) {
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

	additionalBandwidth := int64((factor * float64(vm.Plan.Bandwidth) + 15) * 1024 * 1024 * 1024)
	rows := db.Query("SELECT id FROM region_bandwidth WHERE region = ? AND user_id = ?", vm.Region, vm.UserId)
	if rows.Next() {
		var rowId int
		rows.Scan(&rowId)
		rows.Close()
		db.Exec("UPDATE region_bandwidth SET bandwidth_additional = bandwidth_additional + ? WHERE id = ?", additionalBandwidth, vm.UserId)
	} else {
		db.Exec("INSERT INTO region_bandwidth (user_id, region, bandwidth_additional) VALUES (?, ?, ?)", vm.UserId, vm.Region, additionalBandwidth)
	}
}

// Attempts to bill on the specified virtual machine.
// terminating should be set to true if the VM is about to be deleted, so that we:
//  a) bill for the last used interval
//  b) enforce BILLING_VM_MINIMUM
func vmBilling(db *Database, vmId int, terminating bool) {
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
				if alreadyBilledIntervals + intervals < cfg.Billing.BillingVmMinimum {
					intervals = cfg.Billing.BillingVmMinimum - alreadyBilledIntervals
				}
			}
		}
	}
	if intervals == 0 {
		return
	}

	vm := vmGet(db, vmId)
	if vm == nil {
		log.Printf("vmBilling: vm id=%d disappeared during execution", vmId)
		return
	} else if vm.Status != "active" {
		return
	}

	amount := int64(intervals) * int64(vm.Plan.Price)
	userApplyCharge(db, vm.UserId, vm.Name, "Plan: " + vm.Plan.Name, fmt.Sprintf("vm-%d", vmId), amount)
	db.Exec("UPDATE vms SET time_billed = DATE_ADD(time_billed, INTERVAL ? MINUTE) WHERE id = ?", intervals * cfg.Billing.BillingInterval, vmId)

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
			db.Exec("INSERT INTO region_bandwidth (user_id, region, bandwidth_used) VALUES (?, ?, ?)", vm.UserId, vm.Region, newBytesUsed)
		}
	}
}

// Bills for used storage space and other resources hourly.
func serviceBilling(db *Database) {
	db.Exec("UPDATE users SET time_billed = NOW() WHERE time_billed = 0")
	rows := db.Query("SELECT id, TIMESTAMPDIFF(HOUR, time_billed, NOW()) FROM users WHERE status = 'active' AND TIMESTAMPDIFF(HOUR, time_billed, NOW()) > 0")
	defer rows.Close()

	for rows.Next() {
		var userId, hours int
		rows.Scan(&userId, &hours)
		var hourlyCharge int64 = 0

		// bill storage space
		var storageBytes int64 = 0
		for _, image := range imageList(db, userId) {
			if image.UserId == userId {
				details := imageInfo(db, userId, image.Id)
				if details != nil && details.Info.Size > 0 {
					storageBytes += details.Info.Size
				}
			}
		}
		creditPerGBHour := int64(cfg.Billing.StorageFee * BILLING_PRECISION)
		hourlyCharge += storageBytes * creditPerGBHour / 1000 / 1000 / 1000

		if hourlyCharge > 0 {
			totalCharge := hourlyCharge * int64(hours)
			log.Printf("Charging user %d for %d bytes (amount=%.5f)", userId, storageBytes, float64(totalCharge) / BILLING_PRECISION)
			userApplyCharge(db, userId, "Image storage space", fmt.Sprintf("%d MB", storageBytes / 1000 / 1000), "storage", totalCharge)
		}

		db.Exec("UPDATE users SET time_billed = DATE_ADD(time_billed, INTERVAL ? HOUR) WHERE id = ?", hours, userId)
	}
}
