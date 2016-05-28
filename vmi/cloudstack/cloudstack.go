package cloudstack

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/ipaddr"

import "errors"
import "fmt"
import "strings"
import "time"

type CloudStack struct {
	client *API
}

func MakeCloudStack(targetURL string, zoneID string, networkID string, apiKey string, secretKey string) *CloudStack {
	cs := new(CloudStack)
	cs.client = &API{
		TargetURL: targetURL,
		ZoneID:    zoneID,
		NetworkID: networkID,
		APIKey:    apiKey,
		SecretKey: secretKey,
	}
	return cs
}

func (cs *CloudStack) findServiceOffering(cpu int, ram int) (string, error) {
	offerings, err := cs.client.ListServiceOfferings()
	if err != nil {
		return "", err
	}
	for _, offering := range offerings {
		if offering.CPUNumber == cpu && offering.Memory == ram {
			return offering.ID, nil
		}
	}
	return "", fmt.Errorf("no service offering with %d vcpus and %d MB RAM", cpu, ram)
}

func (cs *CloudStack) findDiskOffering(size int) (string, error) {
	offerings, err := cs.client.ListDiskOfferings()
	if err != nil {
		return "", err
	}
	for _, offering := range offerings {
		if offering.DiskSize == size {
			return offering.ID, nil
		}
	}
	return "", fmt.Errorf("no disk offering with %d GB space", size)
}

func (cs *CloudStack) VmCreate(vm *lobster.VirtualMachine, options *lobster.VMIVmCreateOptions) (string, error) {
	var serviceOfferingID, diskOfferingID string
	var err error

	if vm.Plan.Identification == "" {
		serviceOfferingID, err = cs.findServiceOffering(vm.Plan.Cpu, vm.Plan.Ram)
		if err != nil {
			return "", err
		}
		diskOfferingID, err = cs.findDiskOffering(vm.Plan.Storage)
		if err != nil {
			return "", err
		}
	} else {
		parts := strings.Split(vm.Plan.Identification, ":")
		if len(parts) != 2 {
			return "", errors.New("plan identification does not contain two colon-separated parts")
		}
		serviceOfferingID = parts[0]
		diskOfferingID = parts[1]
	}

	id, jobid, err := cs.client.DeployVirtualMachine(serviceOfferingID, diskOfferingID, options.ImageIdentification)
	if err != nil {
		return "", err
	}

	vm.SetMetadata("password", "pending")
	go func() {
		deadline := time.Now().Add(time.Minute)
		for time.Now().Before(deadline) {
			time.Sleep(5 * time.Second)
			result, _ := cs.client.QueryDeployJob(jobid)
			if result != nil {
				vm.SetMetadata("password", result.Password)
				return
			}
		}

		// time limit exceeded
		vm.SetMetadata("password", "unknown")
	}()

	return id, nil
}

func (cs *CloudStack) VmDelete(vm *lobster.VirtualMachine) error {
	return cs.client.DestroyVirtualMachine(vm.Identification, true)
}

func (cs *CloudStack) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	details, err := cs.client.GetVirtualMachine(vm.Identification)
	if err != nil {
		return nil, err
	}

	status := details.State
	if status == "Running" {
		status = "Online"
	} else if status == "Stopped" {
		status = "Offline"
	}

	info := lobster.VmInfo{
		Status:       status,
		LoginDetails: "password: " + vm.Metadata("password", "unknown"),
	}

	for _, nic := range details.Nics {
		if ipaddr.IsPrivate(nic.Addr) {
			info.PrivateIp = nic.Addr
		} else {
			info.Ip = nic.Addr
		}
	}

	return &info, nil
}

func (cs *CloudStack) VmStart(vm *lobster.VirtualMachine) error {
	return cs.client.StartVirtualMachine(vm.Identification)
}

func (cs *CloudStack) VmStop(vm *lobster.VirtualMachine) error {
	return cs.client.StopVirtualMachine(vm.Identification)
}

func (cs *CloudStack) VmReboot(vm *lobster.VirtualMachine) error {
	return cs.client.RebootVirtualMachine(vm.Identification)
}

func (cs *CloudStack) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	return errors.New("operation not supported")
}

func (cs *CloudStack) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	return cs.client.CreateVMSnapshot(vm.Identification)
}

func (cs *CloudStack) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	return 0
}
