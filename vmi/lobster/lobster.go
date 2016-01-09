package lobster

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/api"
import "github.com/LunaNode/lobster/utils"

import "errors"
import "fmt"
import "strconv"

type Lobster struct {
	region       string
	client       *api.Client
	canVnc       bool
	canReimage   bool
	canSnapshot  bool
	canAddresses bool
}

func MakeLobster(region string, url string, apiId string, apiKey string) *Lobster {
	this := new(Lobster)
	this.region = region
	this.client = &api.Client{
		Url:    url,
		ApiId:  apiId,
		ApiKey: apiKey,
	}

	return this
}

func (this *Lobster) findMatchingPlan(ram int, storage int, cpu int) (*api.Plan, error) {
	plans, err := this.client.PlanList()
	if err != nil {
		return nil, err
	}
	for _, plan := range plans {
		if plan.Ram == ram && plan.Storage == storage && plan.Cpu == cpu {
			return plan, nil
		}
	}
	return nil, nil
}

func (this *Lobster) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	var plan int
	if vm.Plan.Identification != "" {
		plan, _ = strconv.Atoi(vm.Plan.Identification)
	} else {
		matchPlan, err := this.findMatchingPlan(vm.Plan.Ram, vm.Plan.Storage, vm.Plan.Cpu)
		if err != nil {
			return "", err
		} else if matchPlan == nil {
			return "", errors.New("plan not available in this region")
		}
		plan = matchPlan.Id
	}

	imageId, _ := strconv.Atoi(imageIdentification)
	vmId, err := this.client.VmCreate(vm.Name, plan, imageId)
	return fmt.Sprintf("%d", vmId), err
}

func (this *Lobster) VmDelete(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmDelete(vmIdentification)
}

func (this *Lobster) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	apiInfoResponse, err := this.client.VmInfo(vmIdentification)
	if err != nil {
		return nil, err
	}

	apiInfo := apiInfoResponse.Details
	info := lobster.VmInfo{
		Ip:                   apiInfo.Ip,
		PrivateIp:            apiInfo.PrivateIp,
		Status:               apiInfo.Status,
		Hostname:             apiInfo.Hostname,
		BandwidthUsed:        apiInfo.BandwidthUsed,
		LoginDetails:         apiInfo.LoginDetails,
		Details:              apiInfo.Details,
		OverrideCapabilities: true,
		CanVnc:               apiInfo.CanVnc,
		CanReimage:           apiInfo.CanReimage,
		CanResize:            apiInfo.CanResize,
		CanSnapshot:          apiInfo.CanSnapshot,
		CanAddresses:         apiInfo.CanAddresses,
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

	return &info, nil
}

func (this *Lobster) VmStart(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAction(vmIdentification, "start", "")
}

func (this *Lobster) VmStop(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAction(vmIdentification, "stop", "")
}

func (this *Lobster) VmReboot(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAction(vmIdentification, "reboot", "")
}

func (this *Lobster) VmVnc(vm *lobster.VirtualMachine) (string, error) {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmVnc(vmIdentification)
}

func (this *Lobster) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAction(vmIdentification, action, value)
}

func (this *Lobster) VmRename(vm *lobster.VirtualMachine, name string) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAction(vmIdentification, "rename", name)
}

func (this *Lobster) CanRename() bool {
	return true
}

func (this *Lobster) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	imageIdentificationInt, _ := strconv.Atoi(imageIdentification)
	return this.client.VmReimage(vmIdentification, imageIdentificationInt)
}

func (this *Lobster) VmResize(vm *lobster.VirtualMachine, plan *lobster.Plan) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	matchPlan, err := this.findMatchingPlan(plan.Ram, plan.Storage, plan.Cpu)
	if err != nil {
		return err
	} else if matchPlan == nil {
		return errors.New("plan not available in this region")
	}
	return this.client.VmResize(vmIdentification, matchPlan.Id)
}

func (this *Lobster) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	imageId, err := this.client.VmSnapshot(vmIdentification, utils.Uid(16))
	return fmt.Sprintf("%d", imageId), err
}

func (this *Lobster) VmAddresses(vm *lobster.VirtualMachine) ([]*lobster.IpAddress, error) {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	apiAddresses, err := this.client.VmAddresses(vmIdentification)
	if err != nil {
		return nil, err
	}

	var addresses []*lobster.IpAddress
	for _, srcAddress := range apiAddresses {
		dstAddress := new(lobster.IpAddress)
		dstAddress.Ip = srcAddress.Ip
		dstAddress.PrivateIp = srcAddress.PrivateIp
		dstAddress.CanRdns = srcAddress.CanRdns
		dstAddress.Hostname = srcAddress.Hostname
		addresses = append(addresses, dstAddress)
	}
	return addresses, nil
}

func (this *Lobster) VmAddAddress(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAddressAdd(vmIdentification)
}

func (this *Lobster) VmRemoveAddress(vm *lobster.VirtualMachine, ip string, privateip string) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAddressRemove(vmIdentification, ip, privateip)
}

func (this *Lobster) VmSetRdns(vm *lobster.VirtualMachine, ip string, hostname string) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	return this.client.VmAddressRdns(vmIdentification, ip, hostname)
}

func (this *Lobster) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	info, err := this.VmInfo(vm)
	if err == nil {
		return info.BandwidthUsed
	} else {
		return 0
	}
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
	imageIdentificationInt, _ := strconv.Atoi(imageIdentification)
	apiInfoResponse, err := this.client.ImageInfo(imageIdentificationInt)
	if err != nil {
		return nil, err
	}

	apiInfo := apiInfoResponse.Details
	info := lobster.ImageInfo{
		Size:    apiInfo.Size,
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
	imageIdentificationInt, _ := strconv.Atoi(imageIdentification)
	return this.client.ImageDelete(imageIdentificationInt)
}

func (this *Lobster) ImageList() ([]*lobster.Image, error) {
	apiImages, err := this.client.ImageList()
	if err != nil {
		return nil, err
	}
	var images []*lobster.Image
	for _, apiImage := range apiImages {
		if apiImage.Region == this.region {
			images = append(images, &lobster.Image{
				Name:           apiImage.Name,
				Identification: fmt.Sprintf("%d", apiImage.Id),
			})
		}
	}
	return images, nil
}

func (this *Lobster) PlanList() ([]*lobster.Plan, error) {
	apiPlans, err := this.client.PlanList()
	if err != nil {
		return nil, err
	}
	plans := make([]*lobster.Plan, len(apiPlans))
	for i, apiPlan := range apiPlans {
		plans[i] = &lobster.Plan{
			Name:           apiPlan.Name,
			Ram:            apiPlan.Ram,
			Cpu:            apiPlan.Cpu,
			Storage:        apiPlan.Storage,
			Bandwidth:      apiPlan.Bandwidth,
			Identification: fmt.Sprintf("%d", apiPlan.Id),
		}
	}
	return plans, nil
}
