package vultr

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/utils"
import vultr "github.com/LunaNode/vultr/lib"

import "errors"
import "fmt"
import "strconv"
import "strings"

type Vultr struct {
	regionId int
	client *vultr.Client

	vmBandwidth map[string]int64 // for bandwidth accounting
}

func MakeVultr(apiKey string, regionId int) *Vultr {
	this := new(Vultr)
	this.regionId = regionId
	this.client = vultr.NewClient(apiKey, nil)
	return this
}

func (this *Vultr) findMatchingPlan(plan lobster.Plan) (int, error) {
	apiPlans, err := this.client.GetPlans()
	if err != nil {
		return 0, err
	}
	regionPlanIds, err := this.client.GetAvailablePlansForRegion(this.regionId)
	if err != nil {
		return 0, err
	}
	regionPlans := make(map[int]bool)
	for _, planId := range regionPlanIds {
		regionPlans[planId] = true
	}

	for _, apiPlan := range apiPlans {
		if regionPlans[apiPlan.ID] && apiPlan.RAM == plan.Ram && apiPlan.VCpus == plan.Cpu && apiPlan.Disk == plan.Storage {
			return apiPlan.ID, nil
		}
	}

	return 0, errors.New("no matching plan found")
}

func (this *Vultr) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	var planId int
	if vm.Plan.Identification != "" {
		planId, _ = strconv.Atoi(vm.Plan.Identification)
	} else {
		var err error
		planId, err = this.findMatchingPlan(vm.Plan)
		if err != nil {
			return "", err
		}
	}

	serverOptions := &vultr.ServerOptions{
		PrivateNetworking: true,
		IPV6: true,
	}

	imageParts := strings.SplitN(imageIdentification, ":", 2)
	if len(imageParts) != 2 {
		return "", errors.New("malformed image identification: missing colon")
	}
	if imageParts[0] == "iso" {
		serverOptions.ISO, _ = strconv.Atoi(imageParts[1])
	} else if imageParts[0] == "os" {
		serverOptions.OS, _ = strconv.Atoi(imageParts[1])
	} else if imageParts[0] == "snapshot" {
		serverOptions.Snapshot = imageParts[1]
	} else {
		return "", errors.New("invalid image type " + imageParts[0])
	}

	server, err := this.client.CreateServer(vm.Name, this.regionId, planId, serverOptions)
	if err != nil {
		return "", err
	} else {
		return server.ID, nil
	}
}

func (this *Vultr) VmDelete(vm *lobster.VirtualMachine) error {
	return this.client.DeleteServer(vm.Identification)
}

func (this *Vultr) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	server, err := this.client.GetServer(vm.Identification)
	if err != nil {
		return nil, err
	}

	info := lobster.VmInfo{
		Ip: server.MainIP,
		PrivateIp: server.InternalIP,
		Hostname: server.Name,
		BandwidthUsed: int64(server.CurrentBandwidth * 1024 * 1024 * 1024),
		LoginDetails: "password: " + server.DefaultPassword,
	}

	if server.Status == "pending" {
		info.Status = "Installing"
	} else if server.Status == "active" {
		if server.PowerStatus == "stopped" {
			info.Status = "Offline"
		} else if server.PowerStatus == "running" {
			info.Status = "Online"
		} else {
			info.Status = server.PowerStatus
		}
	} else {
		info.Status = fmt.Sprintf("%s (%s)", strings.Title(server.Status), strings.Title(server.PowerStatus))
	}

	return &info, nil
}

func (this *Vultr) VmStart(vm *lobster.VirtualMachine) error {
	return this.client.StartServer(vm.Identification)
}

func (this *Vultr) VmStop(vm *lobster.VirtualMachine) error {
	return this.client.HaltServer(vm.Identification)
}

func (this *Vultr) VmReboot(vm *lobster.VirtualMachine) error {
	return this.client.RebootServer(vm.Identification)
}

func (this *Vultr) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *Vultr) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	snapshot, err := this.client.CreateSnapshot(vm.Identification, utils.Uid(16))
	if err != nil {
		return "", err
	} else {
		return "snapshot:" + snapshot.ID, nil
	}
}

func (this *Vultr) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
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

func (this *Vultr) ImageFetch(url string, format string) (string, error) {
	return "", errors.New("operation not supported")
}

func (this *Vultr) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	imageParts := strings.SplitN(imageIdentification, ":", 2)
	if len(imageParts) != 2 {
		return nil, errors.New("malformed image identification: missing colon")
	} else if imageParts[0] != "snapshot" {
		return nil, errors.New("can only fetch info for snapshot images")
	}
	snapshots, err := this.client.GetSnapshots()
	if err != nil {
		return nil, err
	}
	for _, snapshot := range snapshots {
		if snapshot.ID == imageParts[1] {
			if snapshot.Status == "complete" {
				return &lobster.ImageInfo {
					Status: lobster.ImageActive,
					Size: snapshot.Size,
				}, nil
			} else {
				return &lobster.ImageInfo {
					Status: lobster.ImagePending,
				}, nil
			}
		}
	}
	return nil, errors.New("image not found")
}

func (this *Vultr) ImageDelete(imageIdentification string) error {
	imageParts := strings.SplitN(imageIdentification, ":", 2)
	if len(imageParts) != 2 {
		return errors.New("malformed image identification: missing colon")
	} else if imageParts[0] != "snapshot" {
		return errors.New("can only delete snapshot images")
	}
	return this.client.DeleteSnapshot(imageParts[1])
}

func (this *Vultr) PlanList() ([]*lobster.Plan, error) {
	apiPlans, err := this.client.GetPlans()
	if err != nil {
		return nil, err
	}
	regionPlanIds, err := this.client.GetAvailablePlansForRegion(this.regionId)
	if err != nil {
		return nil, err
	}
	regionPlans := make(map[int]bool)
	for _, planId := range regionPlanIds {
		regionPlans[planId] = true
	}

	var plans []*lobster.Plan
	for _, apiPlan := range apiPlans {
		if regionPlans[apiPlan.ID] {
			plan := &lobster.Plan{
				Name: apiPlan.Name,
				Ram: apiPlan.RAM,
				Cpu: apiPlan.VCpus,
				Storage: apiPlan.Disk,
				Identification: fmt.Sprintf("%d", apiPlan.ID),
			}
			plan.Bandwidth, _ = strconv.Atoi(apiPlan.Bandwidth)
			plans = append(plans, plan)
		}
	}
	return plans, nil
}
