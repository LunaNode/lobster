package digitalocean

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/utils"
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
		return fmt.Sprintf("%dgb", ram/1024)
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
	password := utils.Uid(16)
	image, err := this.findImage(imageIdentification)
	if err != nil {
		return "", err
	}

	plan := vm.Plan.Identification
	if plan == "" {
		plan = this.getPlanName(vm.Plan.Ram)
	}

	createRequest := &godo.DropletCreateRequest{
		Name:   vm.Name,
		Region: this.region,
		Size:   plan,
		Image: godo.DropletCreateImage{
			ID: image.ID,
		},
		IPv6:              true,
		PrivateNetworking: true,
		UserData:          fmt.Sprintf("#cloud-config\nchpasswd:\n list: |\n  root:%s\n expire: False\n", password),
	}
	droplet, _, err := this.client.Droplets.Create(createRequest)
	if err != nil {
		return "", err
	} else {
		vm.SetMetadata("password", password)
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
		Hostname:     droplet.Name,
		LoginDetails: "username: root; password: " + vm.Metadata("password", "unknown"),
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

	// list droplet actions
	var pendingActions []string
	actionList, _, err := this.client.Droplets.Actions(droplet.ID, &godo.ListOptions{PerPage: 25})
	if err == nil {
		for _, action := range actionList {
			if action.Status == "in-progress" {
				pendingActions = append(pendingActions, action.Type)
			}
		}
		if len(pendingActions) >= 1 {
			info.Details = make(map[string]string)
			if len(pendingActions) == 1 {
				info.Details["Pending action"] = pendingActions[0]
			} else {
				info.Details["Pending actions"] = strings.Join(pendingActions, ", ")
			}
		}
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

func (this *DigitalOcean) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	vmIdentification, _ := strconv.Atoi(vm.Identification)
	snapshotName := fmt.Sprintf("%d.%s", vm.Id, utils.Uid(16))
	action, _, err := this.client.DropletActions.Snapshot(vmIdentification, snapshotName)
	if err != nil {
		return "", err
	}
	err = this.processAction(vm.Id, action.ID)
	if err != nil {
		return "", err
	} else {
		return "snapshot:" + snapshotName, nil
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

func (this *DigitalOcean) findImage(imageIdentification string) (*godo.Image, error) {
	parts := strings.SplitN(imageIdentification, ":", 2)
	if len(parts) == 2 {
		if parts[0] == "snapshot" {
			images, _, err := this.client.Images.ListUser(&godo.ListOptions{PerPage: 500})
			if err != nil {
				return nil, err
			}
			for _, image := range images {
				if image.Name == parts[1] {
					return &image, nil
				}
			}
			return nil, errors.New("could not find image on account")
		} else {
			return nil, errors.New("invalid image prefix " + parts[0])
		}
	} else {
		image, _, err := this.client.Images.GetBySlug(imageIdentification)
		return image, err
	}
}

func (this *DigitalOcean) ImageFetch(url string, format string) (string, error) {
	return "", errors.New("operation not supported")
}

func (this *DigitalOcean) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	image, err := this.findImage(imageIdentification)
	if err != nil {
		if strings.Contains(err.Error(), "could not find image") {
			return &lobster.ImageInfo{
				Status: lobster.ImagePending,
			}, nil
		} else {
			return nil, err
		}
	}
	return &lobster.ImageInfo{
		Status: lobster.ImageActive,
		Size:   int64(image.MinDiskSize) * 1024 * 1024 * 1024,
	}, nil
}

func (this *DigitalOcean) ImageDelete(imageIdentification string) error {
	image, err := this.findImage(imageIdentification)
	if err != nil {
		return err
	}
	_, err = this.client.Images.Delete(image.ID)
	return err
}

func (this *DigitalOcean) ImageList() ([]*lobster.Image, error) {
	apiImages, _, err := this.client.Images.ListDistribution(&godo.ListOptions{})
	if err != nil {
		return nil, err
	}
	images := make([]*lobster.Image, len(apiImages))
	for i, apiImage := range apiImages {
		images[i] = &lobster.Image{
			Name:           fmt.Sprintf("%s %s", apiImage.Distribution, apiImage.Name),
			Identification: apiImage.Slug,
		}
	}
	return images, nil
}

func (this *DigitalOcean) PlanList() ([]*lobster.Plan, error) {
	sizes, _, err := this.client.Sizes.List(&godo.ListOptions{})
	if err != nil {
		return nil, err
	}
	plans := make([]*lobster.Plan, len(sizes))
	for i, size := range sizes {
		plans[i] = &lobster.Plan{
			Name:           size.Slug,
			Ram:            size.Memory,
			Cpu:            size.Vcpus,
			Storage:        size.Disk,
			Bandwidth:      int(size.Transfer * 1024),
			Identification: size.Slug,
		}
	}
	return plans, nil
}
