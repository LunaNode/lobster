package vmdigitalocean

import "github.com/LunaNode/lobster"
import "github.com/digitalocean/godo"
import "golang.org/x/oauth2"

import "errors"
import "fmt"
import "strconv"
import "strings"
import "time"

type TokenSource struct {
	AccessToken string
}
func (t *TokenSource) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: t.AccessToken,
	}
	return token, nil
}

type DigitalOcean struct {
	region string
	client *godo.Client
}

func MakeDigitalOcean(region string, token string) *DigitalOcean {
	this := new(DigitalOcean)
	this.region = region
	tokenSource := &TokenSource{
		AccessToken: token,
	}
	oauthClient := oauth2.NewClient(oauth2.NoContext, tokenSource)
	this.client = godo.NewClient(oauthClient)
	return this
}

func (this *DigitalOcean) getPlanName(ram int) string {
	if ram < 1024 {
		return fmt.Sprintf("%dmb", ram)
	} else {
		return fmt.Sprintf("%dgb", ram / 1024)
	}
}

func (this *DigitalOcean) processAction(vmIdentification int, actionId int) error {
	for i := 0; i < 10; i++ {
		action, _, err := this.client.DropletActions.Get(vmIdentification, actionId)
		if err != nil {
			return err
		} else if action.Status == "completed" {
			return nil
		} else if action.Status != "in-progress" {
			return errors.New("action status is " + action.Status)
		}
		time.Sleep(time.Second)
	}

	// still not done after 10 seconds?
	// let's just assume it will eventually complete...
	return nil
}

func (this *DigitalOcean) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	createRequest := &godo.DropletCreateRequest{
		Name: vm.Name,
		Region: this.region,
		Size: this.getPlanName(vm.Plan.Ram),
		Image: godo.DropletCreateImage{
			Slug: imageIdentification,
		},
		IPv6: true,
		PrivateNetworking: true,
	}
	droplet, _, err := this.client.Droplets.Create(createRequest)
	if err != nil {
		return "", err
	} else {
		return fmt.Sprintf("%d", droplet.ID), nil
	}
}

func (this *DigitalOcean) VmDelete(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	_, err := this.client.Droplets.Delete(vmIdentification)
	return err
}

func (this *DigitalOcean) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	droplet, _, err := this.client.Droplets.Get(vmIdentification)
	if err != nil {
		return nil, err
	}

	info := lobster.VmInfo{
		Hostname: droplet.Name,
	}
	for _, addr4 := range droplet.Networks.V4 {
		if addr4.Type == "public" {
			info.Ip = addr4.IPAddress
		} else if addr4.Type == "private" {
			info.PrivateIp = addr4.IPAddress
		}
	}
	if droplet.Status == "active" {
		info.Status = "Online"
	} else if droplet.Status == "off" {
		info.Status = "Offline"
	} else {
		info.Status = strings.Title(droplet.Status)
	}

	return &info, nil
}

func (this *DigitalOcean) VmStart(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	action, _, err := this.client.DropletActions.PowerOn(vmIdentification)
	if err != nil {
		return err
	} else {
		return this.processAction(vm.Id, action.ID)
	}
}

func (this *DigitalOcean) VmStop(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	action, _, err := this.client.DropletActions.PowerOff(vmIdentification)
	if err != nil {
		return err
	} else {
		return this.processAction(vm.Id, action.ID)
	}
}

func (this *DigitalOcean) VmReboot(vm *lobster.VirtualMachine) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	action, _, err := this.client.DropletActions.Reboot(vmIdentification)
	if err != nil {
		return err
	} else {
		return this.processAction(vm.Id, action.ID)
	}
}

func (this *DigitalOcean) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *DigitalOcean) VmRename(vm *lobster.VirtualMachine, name string) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	action, _, err := this.client.DropletActions.Rename(vmIdentification, name)
	if err != nil {
		return err
	} else {
		return this.processAction(vm.Id, action.ID)
	}
}

func (this *DigitalOcean) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	action, _, err := this.client.DropletActions.RebuildByImageSlug(vmIdentification, imageIdentification)
	if err != nil {
		return err
	} else {
		return this.processAction(vm.Id, action.ID)
	}
}

func (this *DigitalOcean) VmResize(vm *lobster.VirtualMachine, plan *lobster.Plan) error {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	action, _, err := this.client.DropletActions.Resize(vmIdentification, this.getPlanName(plan.Ram), true)
	if err != nil {
		return err
	} else {
		return this.processAction(vm.Id, action.ID)
	}
}

func (this *DigitalOcean) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	return 0
}
