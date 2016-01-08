package solusvm

import "github.com/LunaNode/lobster"

import "errors"
import "fmt"
import "net/http"
import "strconv"
import "strings"

type SolusVM struct {
	VirtType string
	NodeGroup string
	Api *API

	vmBandwidth map[string]int64 // for bandwidth accounting
	setupConsolePage bool
}

func (this *SolusVM) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	name := vm.Name
	if len(name) < 4 {
		name += ".example.com"
	}

	vmId, password, err := this.Api.VmCreate(this.VirtType, this.NodeGroup, name, imageIdentification, vm.Plan.Ram, vm.Plan.Storage, vm.Plan.Cpu)
	vm.SetMetadata("password", password)
	return fmt.Sprintf("%d", vmId), err
}

func (this *SolusVM) VmDelete(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmDelete(vmIdentificationInt)
}

func (this *SolusVM) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	apiInfo, err := this.Api.VmInfo(vmIdentificationInt)
	if err != nil {
		return nil, err
	}

	bwUsed, _ := strconv.ParseInt(strings.Split(apiInfo.Bandwidth, ",")[1], 10, 64)
	info := lobster.VmInfo{
		Ip: apiInfo.Ip,
		PrivateIp: apiInfo.InternalIps,
		Status: strings.Title(apiInfo.State),
		BandwidthUsed: bwUsed,
		LoginDetails: "username: root; password: " + vm.Metadata("password", "unknown"),
	}

	if this.VirtType == "openvz" {
		info.Actions = append(info.Actions, &lobster.VmActionDescriptor{
			Action: "tuntap",
			Name: "TUN/TAP",
			Description: "Enable or disable TUN/TAP.",
			Options: map[string]string{
				"enable": "On",
				"disable": "Off",
			},
		})
	}

	return &info, nil
}

func (this *SolusVM) VmStart(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmStart(vmIdentificationInt)
}

func (this *SolusVM) VmStop(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmStop(vmIdentificationInt)
}

func (this *SolusVM) VmReboot(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmReboot(vmIdentificationInt)
}

func (this *SolusVM) VmVnc(vm *lobster.VirtualMachine) (string, error) {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)

	if this.VirtType == "kvm" || this.VirtType == "xen" {
		vncInfo, err := this.Api.VmVnc(vmIdentificationInt)
		if err != nil {
			return "", err
		} else {
			return lobster.HandleWebsockify(vncInfo.Ip + vncInfo.Port, vncInfo.Password), nil
		}
	} else {
		consoleInfo, err := this.Api.VmConsole(vmIdentificationInt)
		if err != nil {
			return "", err
		} else {
			return lobster.HandleWssh(consoleInfo.Ip + ":" + consoleInfo.Port, consoleInfo.Username, consoleInfo.Password), nil
		}
	}
}

type ConsoleParams struct {
	Frame lobster.FrameParams
	Host string
	Port string
	Username string
	Password string
}

func (this *SolusVM) handleConsole(w http.ResponseWriter, r *http.Request, db *lobster.Database, session *lobster.Session, frameParams lobster.FrameParams) {
	params := ConsoleParams{}
	params.Frame = frameParams
	params.Host = r.URL.Query().Get("host")
	params.Port = r.URL.Query().Get("port")
	params.Username = r.URL.Query().Get("username")
	params.Password = r.URL.Query().Get("password")
	lobster.RenderTemplate(w, "panel", "solusvm_console", params)
}

func (this *SolusVM) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	if action == "tuntap" {
		return this.Api.VmTunTap(vmIdentificationInt, value == "enable")
	} else {
		return errors.New("operation not supported")
	}
}

func (this *SolusVM) VmRename(vm *lobster.VirtualMachine, name string) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmHostname(vmIdentificationInt, name)
}

func (this *SolusVM) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmReimage(vmIdentificationInt, imageIdentification)
}

func (this *SolusVM) VmResize(vm *lobster.VirtualMachine, plan *lobster.Plan) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)

	// we start with disk since that is the most likely one to have problem
	err := this.Api.VmResizeDisk(vmIdentificationInt, plan.Storage)
	if err != nil {
		return err
	}
	err = this.Api.VmResizeMemory(vmIdentificationInt, plan.Ram)
	if err != nil {
		return err
	}
	err = this.Api.VmResizeCpu(vmIdentificationInt, plan.Cpu)
	if err != nil {
		return err
	}
	return nil
}

func (this *SolusVM) VmAddresses(vm *lobster.VirtualMachine) ([]*lobster.IpAddress, error) {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	apiInfo, err := this.Api.VmInfo(vmIdentificationInt)
	if err != nil {
		return nil, err
	}

	var addresses []*lobster.IpAddress
	for _, addrString := range strings.Split(apiInfo.Ips, ",") {
		addrString = strings.TrimSpace(addrString)
		if addrString != "" {
			addresses = append(addresses, &lobster.IpAddress{
				Ip: addrString,
			})
		}
	}
	return addresses, nil
}

func (this *SolusVM) VmAddAddress(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmAddAddress(vmIdentificationInt)
}

func (this *SolusVM) VmRemoveAddress(vm *lobster.VirtualMachine, ip string, privateip string) error {
	// verify ip is on the virtual machine
	addresses, err := this.VmAddresses(vm)
	if err != nil {
		return err
	}
	found := false
	for _, address := range addresses {
		if address.Ip == ip {
			found = true
			break
		}
	}
	if !found {
		return errors.New("invalid IP address")
	}

	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.Api.VmRemoveAddress(vmIdentificationInt, ip)
}

func (this *SolusVM) VmSetRdns(vm *lobster.VirtualMachine, ip string, hostname string) error {
	return errors.New("operation not supported")
}

func (this *SolusVM) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	info, err := this.VmInfo(vm)
	if err != nil {
		return 0
	}

	if this.vmBandwidth == nil {
		this.vmBandwidth = make(map[string]int64)
	}

	currentBandwidth, ok := this.vmBandwidth[vm.Identification]
	this.vmBandwidth[vm.Identification] = info.BandwidthUsed
	if !ok {
		return 0
	} else {
		return info.BandwidthUsed - currentBandwidth
	}
}

func (this *SolusVM) ImageList() ([]*lobster.Image, error) {
	return nil, nil
}
