package solusvm

import "lobster"

import "errors"
import "fmt"
import "strconv"
import "strings"

type SolusVM struct {
	Lobster *lobster.Lobster
	VirtType string
	NodeGroup string
	Api *API

	vmBandwidth map[string]int64 // for bandwidth accounting
}

func (this *SolusVM) VmCreate(name string, plan *lobster.Plan, imageIdentification string) (string, error) {
	if len(name) < 4 {
		name += ".example.com"
	}

	vmId, password, err := this.Api.VmCreate(this.VirtType, this.NodeGroup, name, imageIdentification, plan.Ram, plan.Storage, plan.Cpu)
	return fmt.Sprintf("%d:%s", vmId, password), err
}

func (this *SolusVM) processVmIdentification(vmIdentification string) (int, string) {
	parts := strings.Split(vmIdentification, ":")
	vmIdentificationInt, _ := strconv.ParseInt(parts[0], 10, 32)
	if len(parts) == 1 {
		return int(vmIdentificationInt), ""
	} else {
		return int(vmIdentificationInt), parts[1]
	}
}

func (this *SolusVM) VmDelete(vmIdentification string) error {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	return this.Api.VmDelete(vmIdentificationInt)
}

func (this *SolusVM) VmInfo(vmIdentification string) (*lobster.VmInfo, error) {
	vmIdentificationInt, rootPassword := this.processVmIdentification(vmIdentification)
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
		LoginDetails: "username: root; password: " + rootPassword,
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

func (this *SolusVM) VmStart(vmIdentification string) error {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	return this.Api.VmStart(vmIdentificationInt)
}

func (this *SolusVM) VmStop(vmIdentification string) error {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	return this.Api.VmStop(vmIdentificationInt)
}

func (this *SolusVM) VmReboot(vmIdentification string) error {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	return this.Api.VmReboot(vmIdentificationInt)
}

func (this *SolusVM) VmVnc(vmIdentification string) (string, error) {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	vncInfo, err := this.Api.VmVnc(vmIdentificationInt)
	if err != nil {
		return "", err
	} else {
		if this.Lobster == nil {
			return "", errors.New("solusvm module misconfiguration: lobster instance not referenced")
		}
		return this.Lobster.HandleWebsockify(vncInfo.Ip + vncInfo.Port, vncInfo.Password), nil
	}
}

func (this *SolusVM) CanVnc() bool {
	return true
}

func (this *SolusVM) VmAction(vmIdentification string, action string, value string) error {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	if action == "tuntap" {
		return this.Api.VmTunTap(vmIdentificationInt, value == "enable")
	} else {
		return errors.New("operation not supported")
	}
}

func (this *SolusVM) VmRename(vmIdentification string, name string) error {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	return this.Api.VmHostname(vmIdentificationInt, name)
}

func (this *SolusVM) CanRename() bool {
	return true
}

func (this *SolusVM) VmReimage(vmIdentification string, imageIdentification string) error {
	vmIdentificationInt, _ := this.processVmIdentification(vmIdentification)
	return this.Api.VmReimage(vmIdentificationInt, imageIdentification)
}

func (this *SolusVM) CanReimage() bool {
	return true
}

func (this *SolusVM) BandwidthAccounting(vmIdentification string) int64 {
	info, err := this.VmInfo(vmIdentification)
	if err != nil {
		return 0
	}

	if this.vmBandwidth == nil {
		this.vmBandwidth = make(map[string]int64)
	}

	currentBandwidth, ok := this.vmBandwidth[vmIdentification]
	this.vmBandwidth[vmIdentification] = info.BandwidthUsed
	if !ok {
		return 0
	} else {
		return info.BandwidthUsed - currentBandwidth
	}
}

func (this *SolusVM) CanImages() bool {
	return false
}

func (this *SolusVM) ImageFetch(url string, format string) (string, error) {
	return "", errors.New("operation not supported")
}

func (this *SolusVM) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	return nil, errors.New("operation not supported")
}

func (this *SolusVM) ImageDelete(imageIdentification string) error {
	return errors.New("operation not supported")
}
