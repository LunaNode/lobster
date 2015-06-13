package vmfake

import "github.com/LunaNode/lobster"

import "errors"
import "fmt"
import "math/rand"
import "strings"

type Fake struct {
	Bandwidth int64 // returned on BandwidthAccounting

	CountCreate int
	CountDelete int
	CountInfo int
	CountStart int
	CountStop int
	CountReboot int
	CountVnc int
}

func (this *Fake) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	this.CountCreate++
	vm.SetMetadata("addresses", "127.0.0.1:")
	return "fake", nil
}

func (this *Fake) VmDelete(vm *lobster.VirtualMachine) error {
	this.CountDelete++
	return nil
}

func (this *Fake) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	this.CountInfo++
	info := &lobster.VmInfo{
		Status: "Online",
		LoginDetails: "fingerprint login supported",
	}

	addresses, _ := this.VmAddresses(vm)
	if len(addresses) > 0 {
		info.Ip = addresses[0].Ip
		info.PrivateIp = "255.255.255.255"
	}
	return info, nil
}

func (this *Fake) VmStart(vm *lobster.VirtualMachine) error {
	this.CountStart++
	return nil
}

func (this *Fake) VmStop(vm *lobster.VirtualMachine) error {
	this.CountStop++
	return nil
}

func (this *Fake) VmReboot(vm *lobster.VirtualMachine) error {
	this.CountReboot++
	return nil
}

func (this *Fake) VmVnc(vm *lobster.VirtualMachine) (string, error) {
	this.CountVnc++
	return "https://lunanode.com/", nil
}

func (this *Fake) CanVnc() bool {
	return true
}

func (this *Fake) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *Fake) VmRename(vm *lobster.VirtualMachine, name string) error {
	return nil
}

func (this *Fake) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	return nil
}

func (this *Fake) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	return "fake", nil
}

func (this *Fake) VmAddresses(vm *lobster.VirtualMachine) ([]*lobster.IpAddress, error) {
	var addresses []*lobster.IpAddress
	for _, addrString := range strings.Split(vm.Metadata("addresses", ""), ",") {
		addrString = strings.TrimSpace(addrString)
		if addrString != "" {
			parts := strings.Split(addrString, ":")
			ipAddr := &lobster.IpAddress{
				Ip: parts[0],
				PrivateIp: "255.255.255.255",
				CanRdns: true,
			}
			if len(parts) > 1 {
				ipAddr.Hostname = parts[1]
			}
			addresses = append(addresses, ipAddr)
		}
	}
	return addresses, nil
}

func (this *Fake) saveAddresses(vm *lobster.VirtualMachine, addresses []*lobster.IpAddress) {
	str := ""
	for _, address := range addresses {
		if str != "" {
			str += ","
		}
		str += address.Ip + ":" + address.Hostname
	}
	vm.SetMetadata("addresses", str)
}

func (this *Fake) VmAddAddress(vm *lobster.VirtualMachine) error {
	addresses, _ := this.VmAddresses(vm)
	addresses = append(addresses, &lobster.IpAddress{Ip: "127.0.0." + fmt.Sprintf("%d", rand.Int31n(255) + 1)})
	this.saveAddresses(vm, addresses)
	return nil
}

func (this *Fake) VmRemoveAddress(vm *lobster.VirtualMachine, ip string, privateip string) error {
	addresses, _ := this.VmAddresses(vm)
	var newAddresses []*lobster.IpAddress
	for _, address := range addresses {
		if address.Ip != ip {
			newAddresses = append(newAddresses, address)
		}
	}
	this.saveAddresses(vm, newAddresses)
	return nil
}

func (this *Fake) VmSetRdns(vm *lobster.VirtualMachine, ip string, hostname string) error {
	addresses, _ := this.VmAddresses(vm)
	for _, address := range addresses {
		if address.Ip == ip {
			address.Hostname = hostname
		}
	}
	this.saveAddresses(vm, addresses)
	return nil
}

func (this *Fake) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	return this.Bandwidth
}

func (this *Fake) ImageFetch(url string, format string) (string, error) {
	return "fake", nil
}

func (this *Fake) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	return &lobster.ImageInfo{
		Size: int64(1024 * 1024 * 1024),
		Status: lobster.ImageActive,
	}, nil
}

func (this *Fake) ImageDelete(imageIdentification string) error {
	return nil
}
