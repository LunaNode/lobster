package linode

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/utils"
import "github.com/LunaNode/go-linode"

import "errors"
import "fmt"
import "strconv"
import "strings"

type Linode struct {
	datacenterID int
	client *linode.Client
}

func MakeLinode(apiKey string, datacenterID int) *Linode {
	this := new(Linode)
	this.datacenterID = datacenterID
	this.client = linode.NewClient(apiKey)
	return this
}

func (this *Linode) findMatchingPlan(plan lobster.Plan) (int, error) {
	apiPlans, err := this.client.ListPlans()
	if err != nil {
		return 0, err
	}
	for _, apiPlan := range apiPlans {
		if apiPlan.RAM == plan.Ram && apiPlan.Cores == plan.Cpu && apiPlan.Disk == plan.Storage {
			return apiPlan.ID, nil
		} else {
		}
	}

	return 0, errors.New("no matching plan found")
}

func (this *Linode) findKernel() (int, error) {
	kernels, err := this.client.ListKernels()
	if err != nil {
		return 0, err
	}
	for _, kernel := range kernels {
		if strings.Contains(kernel.Label, "Latest 64 bit") {
			return kernel.ID, nil
		}
	}
	return 0, errors.New("no kernel found")
}

func (this *Linode) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	var planID int
	if vm.Plan.Identification != "" {
		planID, _ = strconv.Atoi(vm.Plan.Identification)
	} else {
		var err error
		planID, err = this.findMatchingPlan(vm.Plan)
		if err != nil {
			return "", err
		}
	}
	kernelID, err := this.findKernel()
	if err != nil {
		return "", err
	}
	password := utils.Uid(16)

	// create linode
	linodeID, err := this.client.CreateLinode(this.datacenterID, planID)
	if err != nil {
		return "", err
	}

	// create disks
	totalDiskMB := vm.Plan.Storage * 1024
	swapSize := vm.Plan.Ram / 2
	diskSize := totalDiskMB - swapSize

	var diskID int
	imageParts := strings.SplitN(imageIdentification, ":", 2)
	if len(imageParts) != 2 {
		return "", errors.New("malformed image identification: missing colon")
	}
	if imageParts[0] == "distribution" {
		distributionID, _ := strconv.Atoi(imageParts[1])
		diskID, _, err = this.client.CreateDiskFromDistribution(linodeID, "lobster", distributionID, diskSize, password, "")
		if err != nil {
			this.client.DeleteLinode(linodeID, false)
			return "", err
		}
	} else if imageParts[0] == "image" {
		imageID, _ := strconv.Atoi(imageParts[1])
		diskID, _, err = this.client.CreateDiskFromImage(linodeID, "lobster", imageID, diskSize, password, "")
		if err != nil {
			this.client.DeleteLinode(linodeID, false)
			return "", err
		}
	} else {
		return "", errors.New("invalid image type " + imageParts[0])
	}

	vm.SetMetadata("diskid", fmt.Sprintf("%d", diskID))
	vm.SetMetadata("password", password)

	swapID, _, err := this.client.CreateDisk(linodeID, "lobster-swap", "swap", swapSize, linode.CreateDiskOptions{})
	if err != nil {
		this.client.DeleteLinode(linodeID, false)
		return "", err
	}

	configID, err := this.client.CreateConfig(linodeID, kernelID, fmt.Sprintf("lobster-%d", vm.Id), []int{diskID, swapID}, linode.CreateConfigOptions{})
	if err != nil {
		this.client.DeleteLinode(linodeID, false)
		return "", err
	} else {
		vm.SetMetadata("configid", fmt.Sprintf("%d", configID))
		this.client.BootLinode(linodeID)
		return fmt.Sprintf("%d", linodeID), nil
	}
}

func (this *Linode) VmDelete(vm *lobster.VirtualMachine) error {
	linodeID, _ := strconv.Atoi(vm.Identification)
	return this.client.DeleteLinode(linodeID, true)
}

func (this *Linode) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	linodeID, _ := strconv.Atoi(vm.Identification)
	linode, err := this.client.GetLinode(linodeID)
	if err != nil {
		return nil, err
	}

	info := lobster.VmInfo{
		Hostname: vm.Name,
		LoginDetails: "password: " + vm.Metadata("password", "unknown"),
	}

	if linode.StatusString == "Running" {
		info.Status = "Online"
	} else if linode.StatusString == "Powered Off" {
		info.Status = "Offline"
	} else {
		info.Status = linode.StatusString
	}

	ips, err := this.client.ListIP(linodeID)
	if err == nil {
		for _, ip := range ips {
			if ip.IsPublic == 1 {
				info.Ip = ip.Address
			} else {
				info.PrivateIp = ip.Address
			}
		}
	}

	return &info, nil
}

func (this *Linode) VmStart(vm *lobster.VirtualMachine) error {
	linodeID, _ := strconv.Atoi(vm.Identification)
	_, err := this.client.BootLinode(linodeID)
	return err
}

func (this *Linode) VmStop(vm *lobster.VirtualMachine) error {
	linodeID, _ := strconv.Atoi(vm.Identification)
	_, err := this.client.ShutdownLinode(linodeID)
	return err
}

func (this *Linode) VmReboot(vm *lobster.VirtualMachine) error {
	linodeID, _ := strconv.Atoi(vm.Identification)
	_, err := this.client.RebootLinode(linodeID)
	return err
}

func (this *Linode) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *Linode) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	linodeID, _ := strconv.Atoi(vm.Identification)
	diskID, err := strconv.Atoi(vm.Metadata("diskid", ""))
	if err != nil {
		return "", errors.New("failed to retrieve disk ID from metadata")
	}
	imageID, _, err := this.client.ImagizeDisk(linodeID, diskID, "lobster image")
	if err != nil {
		return "", err
	} else {
		return fmt.Sprintf("image:%d", imageID), nil
	}
}

func (this *Linode) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	return 0
}

func (this *Linode) ImageFetch(url string, format string) (string, error) {
	return "", errors.New("operation not supported")
}

func (this *Linode) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	imageParts := strings.SplitN(imageIdentification, ":", 2)
	if len(imageParts) != 2 {
		return nil, errors.New("malformed image identification: missing colon")
	} else if imageParts[0] != "image" {
		return nil, errors.New("can only fetch info for images")
	}
	imageID, _ := strconv.Atoi(imageParts[1])
	image, err := this.client.GetImage(imageID)
	if err != nil {
		return nil, err
	}
	imageInfo := &lobster.ImageInfo{
		Status: lobster.ImagePending,
		Size: image.MinSize * 1024 * 1024,
	}
	if image.Status == "available" {
		imageInfo.Status = lobster.ImageActive
	}
	return imageInfo, nil
}

func (this *Linode) ImageDelete(imageIdentification string) error {
	imageParts := strings.SplitN(imageIdentification, ":", 2)
	if len(imageParts) != 2 {
		return errors.New("malformed image identification: missing colon")
	} else if imageParts[0] != "image" {
		return errors.New("can only delete images")
	}
	imageID, _ := strconv.Atoi(imageParts[1])
	return this.client.DeleteImage(imageID)
}

func (this *Linode) PlanList() ([]*lobster.Plan, error) {
	apiPlans, err := this.client.ListPlans()
	if err != nil {
		return nil, err
	}
	plans := make([]*lobster.Plan, len(apiPlans))
	for i, apiPlan := range apiPlans {
		plans[i] = &lobster.Plan{
			Name: apiPlan.Label,
			Ram: apiPlan.RAM,
			Cpu: apiPlan.Cores,
			Storage: apiPlan.Disk,
			Bandwidth: apiPlan.Bandwidth,
			Identification: fmt.Sprintf("%d", apiPlan.ID),
		}
	}
	return plans, nil
}
