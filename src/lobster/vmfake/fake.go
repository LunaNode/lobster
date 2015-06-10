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

func (this *Fake) VmCreate(name string, plan *lobster.Plan, imageIdentification string) (string, error) {
	this.CountCreate++
	return "fake", nil
}

func (this *Fake) VmDelete(vmIdentification string) error {
	this.CountDelete++
	return nil
}

func (this *Fake) VmInfo(vmIdentification string) (*lobster.VmInfo, error) {
	this.CountInfo++
	return &lobster.VmInfo{
		Ip: "127.0.0.1",
		PrivateIp: "255.255.255.255",
		Status: "Online",
		LoginDetails: "fingerprint login supported",
	}, nil
}

func (this *Fake) VmStart(vmIdentification string) error {
	this.CountStart++
	return nil
}

func (this *Fake) VmStop(vmIdentification string) error {
	this.CountStop++
	return nil
}

func (this *Fake) VmReboot(vmIdentification string) error {
	this.CountReboot++
	return nil
}

func (this *Fake) VmVnc(vmIdentification string) (string, error) {
	this.CountVnc++
	return "https://lunanode.com/", nil
}

func (this *Fake) CanVnc() bool {
	return true
}

func (this *Fake) VmAction(vmIdentification string, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *Fake) VmRename(vmIdentification string, name string) error {
	return nil
}

func (this *Fake) CanRename() bool {
	return true
}

func (this *Fake) VmReimage(vmIdentification string, imageIdentification string) error {
	return nil
}

func (this *Fake) CanReimage() bool {
	return true
}

func (this *Fake) BandwidthAccounting(vmIdentification string) int64 {
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
