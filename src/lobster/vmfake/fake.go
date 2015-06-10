package vmfake

import "lobster"

import "errors"

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
	return "fake", nil
}

func (this *Fake) VmDelete(vm *lobster.VirtualMachine) error {
	this.CountDelete++
	return nil
}

func (this *Fake) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	this.CountInfo++
	return &lobster.VmInfo{
		Ip: "127.0.0.1",
		PrivateIp: "255.255.255.255",
		Status: "Online",
		LoginDetails: "fingerprint login supported",
	}, nil
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

func (this *Fake) CanRename() bool {
	return true
}

func (this *Fake) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	return nil
}

func (this *Fake) CanReimage() bool {
	return true
}

func (this *Fake) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	return this.Bandwidth
}

func (this *Fake) CanImages() bool {
	return true
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
