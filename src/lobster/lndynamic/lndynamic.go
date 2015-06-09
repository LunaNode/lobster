package lndynamic

import "lobster"
import "strconv"
import "errors"
import "fmt"

type LNDynamic struct {
	region string
	api *API

	vmBandwidth map[string]int64 // for bandwidth accounting
}

func MakeLNDynamic(region string, apiId string, apiKey string) *LNDynamic {
	this := new(LNDynamic)
	this.region = region
	api, err := MakeAPI(apiId, apiKey)
	if err != nil {
		panic(err)
	} else {
		this.api = api
	}
	return this
}

func (this *LNDynamic) VmCreate(name string, plan *lobster.Plan, imageIdentification string) (string, error) {
	plans, err := this.api.PlanList()
	if err != nil {
		return "", err
	}

	var matchPlan *APIPlan
	for _, apiPlan := range plans {
		cpu, _ := strconv.ParseInt(apiPlan.Vcpu, 10, 32)
		ram, _ := strconv.ParseInt(apiPlan.Ram, 10, 32)
		storage, _ := strconv.ParseInt(apiPlan.Storage, 10, 32)

		if int(cpu) == plan.Cpu && int(ram) == plan.Ram && int(storage) == plan.Storage {
			matchPlan = apiPlan
			break
		}
	}

	if matchPlan == nil {
		return "", errors.New("plan not available in this region")
	}

	planIdentification, _ := strconv.ParseInt(matchPlan.Id, 10, 32)
	imageIdentificationInt, _ := strconv.ParseInt(imageIdentification, 10, 32)
	vmId, err := this.api.VmCreateImage(this.region, name, int(planIdentification), int(imageIdentificationInt))
	return fmt.Sprintf("%d", vmId), err
}

func (this *LNDynamic) VmDelete(vmIdentification string) error {
	vmIdentificationInt, _ := strconv.ParseInt(vmIdentification, 10, 32)
	return this.api.VmDelete(int(vmIdentificationInt))
}

func (this *LNDynamic) VmInfo(vmIdentification string) (*lobster.VmInfo, error) {
	vmIdentificationInt, _ := strconv.ParseInt(vmIdentification, 10, 32)
	apiInfo, err := this.api.VmInfo(int(vmIdentificationInt))
	if err != nil {
		return nil, err
	}

	bwUsed, _ := strconv.ParseFloat(apiInfo.BandwidthUsed, 64)
	info := lobster.VmInfo{
		Ip: apiInfo.Ip,
		PrivateIp: apiInfo.PrivateIp,
		Status: apiInfo.Status,
		Hostname: apiInfo.Hostname,
		BandwidthUsed: int64(bwUsed * 1024 * 1024 * 1024),
		LoginDetails: apiInfo.LoginDetails,
	}
	return &info, nil
}

func (this *LNDynamic) VmStart(vmIdentification string) error {
	vmIdentificationInt, _ := strconv.ParseInt(vmIdentification, 10, 32)
	return this.api.VmStart(int(vmIdentificationInt))
}

func (this *LNDynamic) VmStop(vmIdentification string) error {
	vmIdentificationInt, _ := strconv.ParseInt(vmIdentification, 10, 32)
	return this.api.VmStop(int(vmIdentificationInt))
}

func (this *LNDynamic) VmReboot(vmIdentification string) error {
	vmIdentificationInt, _ := strconv.ParseInt(vmIdentification, 10, 32)
	return this.api.VmReboot(int(vmIdentificationInt))
}

func (this *LNDynamic) VmVnc(vmIdentification string) (string, error) {
	vmIdentificationInt, _ := strconv.ParseInt(vmIdentification, 10, 32)
	return this.api.VmVnc(int(vmIdentificationInt))
}

func (this *LNDynamic) CanVnc() bool {
	return true
}

func (this *LNDynamic) VmAction(vmIdentification string, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *LNDynamic) VmRename(vmIdentification string, name string) error {
	return errors.New("operation not supported")
}

func (this *LNDynamic) CanRename() bool {
	return false
}

func (this *LNDynamic) VmReimage(vmIdentification string, imageIdentification string) error {
	vmIdentificationInt, _ := strconv.ParseInt(vmIdentification, 10, 32)
	imageIdentificationInt, _ := strconv.ParseInt(imageIdentification, 10, 32)
	return this.api.VmReimage(int(vmIdentificationInt), int(imageIdentificationInt))
}

func (this *LNDynamic) CanReimage() bool {
	return true
}

func (this *LNDynamic) BandwidthAccounting(vmIdentification string) int64 {
	info, err := this.VmInfo(vmIdentification)
	if err != nil {
		return 0
	}

	if this.vmBandwidth == nil {
		this.vmBandwidth = make(map[string]int64)
	}

	currentBandwidth, ok := this.vmBandwidth[vmIdentification]
	this.vmBandwidth[vmIdentification] = info.BandwidthUsed
	if !ok || currentBandwidth < info.BandwidthUsed {
		return 0
	} else {
		return info.BandwidthUsed - currentBandwidth
	}
}

func (this *LNDynamic) CanImages() bool {
	return true
}

func (this *LNDynamic) ImageFetch(url string, format string) (string, error) {
	imageId, err := this.api.ImageFetch(this.region, url, format, false)
	if err != nil {
		return "", err
	} else {
		return fmt.Sprintf("%d", imageId), nil
	}
}

func (this *LNDynamic) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	imageIdentificationInt, _ := strconv.ParseInt(imageIdentification, 10, 32)
	image, err := this.api.ImageDetails(int(imageIdentificationInt))
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
			Size: size,
			Status: status,
		}, nil
	}
}

func (this *LNDynamic) ImageDelete(imageIdentification string) error {
	imageIdentificationInt, _ := strconv.ParseInt(imageIdentification, 10, 32)
	return this.api.ImageDelete(int(imageIdentificationInt))
}
