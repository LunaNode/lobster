package vmlobster

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/api"

import "errors"
import "fmt"
import "strconv"

type Lobster struct {
	client *api.Client
	canVnc bool
	canReimage bool
}

func MakeLobster(url string, apiId string, apiKey string) *Lobster {
	this := new(Lobster)
	this.client = &api.Client{
		Url: url,
		ApiId: apiId,
		ApiKey: apiKey,
	}

	// assume capabilities by default, then disable if we see VM not able to do it
	this.canVnc = true
	this.canReimage = true
	return this
}

func (this *Lobster) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	plans, err := this.client.PlanList()
	if err != nil {
		return "", err
	}

	var matchPlan *api.Plan
	for _, plan := range plans {
		if plan.Ram == vm.Plan.Ram && plan.Storage == vm.Plan.Storage && plan.Cpu == vm.Plan.Cpu {
			matchPlan = plan
			break
		}
	}
	if matchPlan == nil {
		return "", errors.New("plan not available in this region")
	}

	imageId, _ := strconv.ParseInt(imageIdentification, 10, 32)
	vmId, err := this.client.VmCreate(vm.Name, matchPlan.Id, int(imageId))
	return fmt.Sprintf("%d", vmId), err
}

func (this *Lobster) VmDelete(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	return this.client.VmDelete(int(vmIdentification))
}

func (this *Lobster) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	apiInfoResponse, err := this.client.VmInfo(int(vmIdentification))
	if err != nil {
		return nil, err
	}

	apiInfo := apiInfoResponse.Details
	info := lobster.VmInfo{
		Ip: apiInfo.Ip,
		PrivateIp: apiInfo.PrivateIp,
		Status: apiInfo.Status,
		Hostname: apiInfo.Hostname,
		BandwidthUsed: apiInfo.BandwidthUsed,
		LoginDetails: apiInfo.LoginDetails,
		Details: apiInfo.Details,
	}
	for _, srcAction := range apiInfo.Actions {
		dstAction := new(lobster.VmActionDescriptor)
		dstAction.Action = srcAction.Action
		dstAction.Name = srcAction.Name
		dstAction.Options = srcAction.Options
		dstAction.Description = srcAction.Description
		dstAction.Dangerous = srcAction.Dangerous
		info.Actions = append(info.Actions, dstAction)
	}

	// set capabilities to false if needed
	if !apiInfo.CanVnc {
		this.canVnc = false
	}
	if !apiInfo.CanReimage {
		this.canReimage = false
	}

	return &info, nil
}

func (this *Lobster) VmStart(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	return this.client.VmAction(int(vmIdentification), "start", "")
}

func (this *Lobster) VmStop(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	return this.client.VmAction(int(vmIdentification), "stop", "")
}

func (this *Lobster) VmReboot(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	return this.client.VmAction(int(vmIdentification), "reboot", "")
}

func (this *Lobster) VmVnc(vm *lobster.VirtualMachine) (string, error) {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	return this.client.VmVnc(int(vmIdentification))
}

func (this *Lobster) CanVnc() bool {
	return this.canVnc
}

func (this *Lobster) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	return this.client.VmAction(int(vmIdentification), action, value)
}

func (this *Lobster) VmRename(vm *lobster.VirtualMachine, name string) error {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	return this.client.VmAction(int(vmIdentification), "rename", name)
}

func (this *Lobster) CanRename() bool {
	return true
}

func (this *Lobster) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	vmIdentification, _ := strconv.ParseInt(vm.Identification, 10, 32)
	imageIdentificationInt, _ := strconv.ParseInt(imageIdentification, 10, 32)
	return this.client.VmReimage(int(vmIdentification), int(imageIdentificationInt))
}

func (this *Lobster) CanReimage() bool {
	return this.canReimage
}

func (this *Lobster) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	info, err := this.VmInfo(vm)
	if err == nil {
		return info.BandwidthUsed
	} else {
		return 0
	}
}

func (this *Lobster) CanImages() bool {
	return false
}

func (this *Lobster) ImageFetch(url string, format string) (string, error) {
	return "", errors.New("operation not supported")
}

func (this *Lobster) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	return nil, errors.New("operation not supporteduu")
}

func (this *Lobster) ImageDelete(imageIdentification string) error {
	return errors.New("operation not supported")
}
