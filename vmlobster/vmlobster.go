package vmlobster

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/api"
import "github.com/LunaNode/lobster/utils"

import "errors"
import "fmt"
import "strconv"

type Lobster struct {
	region string
	client *api.Client
	canVnc bool
	canReimage bool
}

func MakeLobster(region string, url string, apiId string, apiKey string) *Lobster {
	this := new(Lobster)
	this.region = region
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
	return true
}

func (this *Lobster) ImageFetch(url string, format string) (string, error) {
	// backend name doesn't matter, so we create with random string
	imageIdentification, err := this.client.ImageFetch(this.region, utils.Uid(16), url, format)
	if err != nil {
		return "", err
	} else {
		return fmt.Sprintf("%d", imageIdentification), nil
	}
}

func (this *Lobster) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	imageIdentificationInt, _ := strconv.ParseInt(imageIdentification, 10, 32)
	apiInfoResponse, err := this.client.ImageInfo(int(imageIdentificationInt))
	if err != nil {
		return nil, err
	}

	apiInfo := apiInfoResponse.Details
	info := lobster.ImageInfo{
		Size: apiInfo.Size,
		Details: apiInfo.Details,
	}

	if apiInfo.Status == "pending" {
		info.Status = lobster.ImagePending
	} else if apiInfo.Status == "active" {
		info.Status = lobster.ImageActive
	} else if apiInfo.Status == "error" {
		info.Status = lobster.ImageError
	}

	return &info, nil
}

func (this *Lobster) ImageDelete(imageIdentification string) error {
	imageIdentificationInt, _ := strconv.ParseInt(imageIdentification, 10, 32)
	return this.client.ImageDelete(int(imageIdentificationInt))
}
