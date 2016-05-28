package lunanode

// The LunaNode VMI is deprecated.
// Please use the Cloug provider for LunaNode instead.

import "github.com/LunaNode/lobster"

import "errors"
import "fmt"
import "strconv"

type LunaNode struct {
	region string
	api    *API

	vmBandwidth map[string]int64 // for bandwidth accounting
}

func MakeLunaNode(region string, apiId string, apiKey string) *LunaNode {
	this := new(LunaNode)
	this.region = region
	api, err := MakeAPI(apiId, apiKey)
	if err != nil {
		panic(err)
	} else {
		this.api = api
	}
	return this
}

func (this *LunaNode) VmCreate(vm *lobster.VirtualMachine, options *lobster.VMIVmCreateOptions) (string, error) {
	var planIdentification int
	if vm.Plan.Identification != "" {
		planIdentification, _ = strconv.Atoi(vm.Plan.Identification)
	} else {
		plans, err := this.api.PlanList()
		if err != nil {
			return "", err
		}

		var matchPlan *APIPlan
		for _, apiPlan := range plans {
			cpu, _ := strconv.Atoi(apiPlan.Vcpu)
			ram, _ := strconv.Atoi(apiPlan.Ram)
			storage, _ := strconv.Atoi(apiPlan.Storage)

			if cpu == vm.Plan.Cpu && ram == vm.Plan.Ram && storage == vm.Plan.Storage {
				matchPlan = apiPlan
				break
			}
		}

		if matchPlan == nil {
			return "", errors.New("plan not available in this region")
		}
		planIdentification, _ = strconv.Atoi(matchPlan.Id)
	}

	imageIdentificationInt, _ := strconv.Atoi(options.ImageIdentification)
	vmId, err := this.api.VmCreateImage(this.region, vm.Name, planIdentification, imageIdentificationInt)
	return fmt.Sprintf("%d", vmId), err
}

func (this *LunaNode) VmDelete(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.api.VmDelete(vmIdentificationInt)
}

func (this *LunaNode) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	apiInfo, err := this.api.VmInfo(vmIdentificationInt)
	if err != nil {
		return nil, err
	}

	bwUsed, _ := strconv.ParseFloat(apiInfo.BandwidthUsed, 64)
	info := lobster.VmInfo{
		Ip:            apiInfo.Ip,
		PrivateIp:     apiInfo.PrivateIp,
		Status:        apiInfo.Status,
		Hostname:      apiInfo.Hostname,
		BandwidthUsed: int64(bwUsed * 1024 * 1024 * 1024),
		LoginDetails:  apiInfo.LoginDetails,
	}
	return &info, nil
}

func (this *LunaNode) VmStart(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.api.VmStart(vmIdentificationInt)
}

func (this *LunaNode) VmStop(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.api.VmStop(vmIdentificationInt)
}

func (this *LunaNode) VmReboot(vm *lobster.VirtualMachine) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.api.VmReboot(vmIdentificationInt)
}

func (this *LunaNode) VmVnc(vm *lobster.VirtualMachine) (string, error) {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	return this.api.VmVnc(vmIdentificationInt)
}

func (this *LunaNode) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *LunaNode) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	imageIdentificationInt, _ := strconv.Atoi(imageIdentification)
	return this.api.VmReimage(vmIdentificationInt, imageIdentificationInt)
}

func (this *LunaNode) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	vmIdentificationInt, _ := strconv.Atoi(vm.Identification)
	imageId, err := this.api.VmSnapshot(vmIdentificationInt, this.region)
	return fmt.Sprintf("%d", imageId), err
}

func (this *LunaNode) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	info, err := this.VmInfo(vm)
	if err != nil {
		return 0
	}

	if this.vmBandwidth == nil {
		this.vmBandwidth = make(map[string]int64)
	}

	currentBandwidth, ok := this.vmBandwidth[vm.Identification]
	this.vmBandwidth[vm.Identification] = info.BandwidthUsed
	if !ok || currentBandwidth < info.BandwidthUsed {
		return 0
	} else {
		return info.BandwidthUsed - currentBandwidth
	}
}

func (this *LunaNode) ImageFetch(url string, format string) (string, error) {
	imageId, err := this.api.ImageFetch(this.region, url, format, false)
	if err != nil {
		return "", err
	} else {
		return fmt.Sprintf("%d", imageId), nil
	}
}

func (this *LunaNode) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	imageIdentificationInt, _ := strconv.Atoi(imageIdentification)
	image, err := this.api.ImageDetails(imageIdentificationInt)
	if err != nil {
		return nil, err
	} else {
		size, _ := strconv.ParseInt(image.Size, 10, 64)
		status := lobster.ImagePending

		if image.Status == "active" {
			status = lobster.ImageActive
		} else if image.Status == "error" || image.Status == "killed" {
			status = lobster.ImageError
		}

		return &lobster.ImageInfo{
			Size:   size,
			Status: status,
		}, nil
	}
}

func (this *LunaNode) ImageDelete(imageIdentification string) error {
	imageIdentificationInt, _ := strconv.Atoi(imageIdentification)
	return this.api.ImageDelete(imageIdentificationInt)
}

func (this *LunaNode) ImageList() ([]*lobster.Image, error) {
	apiImages, err := this.api.ImageList(this.region)
	if err != nil {
		return nil, err
	}
	images := make([]*lobster.Image, len(apiImages))
	for i, apiImage := range apiImages {
		images[i] = &lobster.Image{
			Name:           apiImage.Name,
			Identification: apiImage.Id,
		}
	}
	return images, nil
}

func (this *LunaNode) PlanList() ([]*lobster.Plan, error) {
	apiPlans, err := this.api.PlanList()
	if err != nil {
		return nil, err
	}
	plans := make([]*lobster.Plan, len(apiPlans))
	for i, apiPlan := range apiPlans {
		plans[i] = &lobster.Plan{
			Name:           apiPlan.Name,
			Identification: apiPlan.Id,
		}
		plans[i].Ram, _ = strconv.Atoi(apiPlan.Ram)
		plans[i].Cpu, _ = strconv.Atoi(apiPlan.Vcpu)
		plans[i].Storage, _ = strconv.Atoi(apiPlan.Storage)
		plans[i].Bandwidth, _ = strconv.Atoi(apiPlan.Bandwidth)
	}
	return plans, nil
}
