package cloug

import "github.com/LunaNode/lobster"

import "github.com/LunaNode/cloug/provider"
import "github.com/LunaNode/cloug/service/compute"

import "encoding/hex"
import "encoding/json"
import "fmt"
import "net/url"
import "strings"

type Cloug struct {
	service compute.Service
	config  Config

	vncService     compute.VNCService
	renameService  compute.RenameService
	reimageService compute.ReimageService
	resizeService  compute.ResizeService
	imageService   compute.ImageService
	addressService compute.AddressService
	flavorService  compute.FlavorService
}

type Config struct {
	Region    string `json:"region"`
	NetworkID string `json:"network_id"`
}

func MakeCloug(jsonData []byte, region string) (*Cloug, error) {
	cloug := new(Cloug)

	err := json.Unmarshal(jsonData, &cloug.config)
	if err != nil {
		return nil, err
	}

	provider, err := provider.ComputeProviderFromJSON(jsonData)
	if err != nil {
		return nil, err
	}
	cloug.service = provider.ComputeService()

	cloug.vncService, _ = cloug.service.(compute.VNCService)
	cloug.renameService, _ = cloug.service.(compute.RenameService)
	cloug.reimageService, _ = cloug.service.(compute.ReimageService)
	cloug.resizeService, _ = cloug.service.(compute.ResizeService)
	cloug.imageService, _ = cloug.service.(compute.ImageService)
	cloug.addressService, _ = cloug.service.(compute.AddressService)
	cloug.flavorService, _ = cloug.service.(compute.FlavorService)

	return cloug, nil
}

func (cloug *Cloug) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	instance, err := cloug.service.CreateInstance(&compute.Instance{
		Name:      vm.Name,
		Region:    cloug.config.Region,
		NetworkID: cloug.config.NetworkID,
		Image:     compute.Image{ID: imageIdentification},
		Flavor: compute.Flavor{
			ID:         vm.Plan.Identification,
			NumCores:   vm.Plan.Cpu,
			DiskGB:     vm.Plan.Storage,
			MemoryMB:   vm.Plan.Ram,
			TransferGB: vm.Plan.Bandwidth,
		},
	})

	if err != nil {
		return "", err
	}

	if instance.Username != "" {
		vm.SetMetadata("username", instance.Username)
	}
	if instance.Password != "" {
		vm.SetMetadata("password", instance.Password)
	}

	return instance.ID, nil
}

func (cloug *Cloug) VmDelete(vm *lobster.VirtualMachine) error {
	return cloug.service.DeleteInstance(vm.Identification)
}

func (cloug *Cloug) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	instance, err := cloug.service.GetInstance(vm.Identification)
	if err != nil {
		return nil, err
	}

	username := vm.Metadata("username", instance.Username)
	if username == "" {
		username = "unknown"
	}
	password := vm.Metadata("password", instance.Password)
	if password == "" {
		password = "unknown"
	}

	info := &lobster.VmInfo{
		Ip:                   instance.IP,
		PrivateIp:            instance.PrivateIP,
		Status:               strings.Title(string(instance.Status)),
		Hostname:             instance.Name,
		BandwidthUsed:        instance.BandwidthUsed,
		LoginDetails:         fmt.Sprintf("username: %s; password: %s", username, password),
		Details:              instance.Details,
		OverrideCapabilities: true,
		CanVnc:               cloug.vncService != nil,
		CanReimage:           cloug.reimageService != nil,
		CanResize:            cloug.resizeService != nil,
		CanSnapshot:          cloug.imageService != nil,
		CanAddresses:         cloug.addressService != nil,
	}

	for _, action := range instance.Actions {
		info.Actions = append(info.Actions, &lobster.VmActionDescriptor{
			Action:      hex.EncodeToString([]byte(action.Label)),
			Name:        action.Label,
			Description: action.Description,
			Options:     action.Options,
		})
	}

	return info, nil
}

func (cloug *Cloug) VmStart(vm *lobster.VirtualMachine) error {
	return cloug.service.StartInstance(vm.Identification)
}

func (cloug *Cloug) VmStop(vm *lobster.VirtualMachine) error {
	return cloug.service.StopInstance(vm.Identification)
}

func (cloug *Cloug) VmReboot(vm *lobster.VirtualMachine) error {
	return cloug.service.RebootInstance(vm.Identification)
}

func (cloug *Cloug) VmVnc(vm *lobster.VirtualMachine) (string, error) {
	if cloug.vncService == nil {
		return "", fmt.Errorf("operation not supported")
	}
	rawurl, err := cloug.vncService.GetVNC(vm.Identification)
	if err != nil {
		return "", err
	}

	// decode URL, depending on protocol we may want to start websockify/wssh
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", fmt.Errorf("failed to parse returned URL: %v", err)
	}

	if u.Scheme == "vnc" {
		return lobster.HandleWebsockify(u.Host, u.Query().Get("password")), nil
	} else if u.Scheme == "ssh" {
		if u.User == nil {
			return "", fmt.Errorf("got URL with ssh scheme, but user info not set")
		}
		username := u.User.Username()
		password, isSet := u.User.Password()
		if !isSet {
			return "", fmt.Errorf("got URL with ssh scheme, but user info does not specify password")
		}
		return lobster.HandleWssh(u.Host, username, password), nil
	} else {
		return rawurl, nil
	}
}

func (cloug *Cloug) VmAction(vm *lobster.VirtualMachine, actionStr string, value string) error {
	instance, err := cloug.service.GetInstance(vm.Identification)
	if err != nil {
		return err
	}

	for _, action := range instance.Actions {
		if hex.EncodeToString([]byte(action.Label)) == actionStr {
			return action.Func(value)
		}
	}

	return fmt.Errorf("unknown action %s", actionStr)
}

func (cloug *Cloug) VmRename(vm *lobster.VirtualMachine, name string) error {
	if cloug.renameService == nil {
		return fmt.Errorf("operation not supported")
	}
	return cloug.renameService.RenameInstance(vm.Identification, name)
}

func (cloug *Cloug) CanRename() bool {
	return cloug.renameService != nil
}

func (cloug *Cloug) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	if cloug.reimageService == nil {
		return fmt.Errorf("operation not supported")
	}
	return cloug.reimageService.ReimageInstance(vm.Identification, &compute.Image{ID: imageIdentification})
}

func (cloug *Cloug) VmResize(vm *lobster.VirtualMachine, plan *lobster.Plan) error {
	if cloug.resizeService == nil {
		return fmt.Errorf("operation not supported")
	}
	return cloug.resizeService.ResizeInstance(vm.Identification, &compute.Flavor{
		ID:         plan.Identification,
		NumCores:   plan.Cpu,
		DiskGB:     plan.Storage,
		MemoryMB:   plan.Ram,
		TransferGB: plan.Bandwidth,
	})
}

func (cloug *Cloug) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	if cloug.imageService == nil {
		return "", fmt.Errorf("operation not supported")
	}
	image, err := cloug.imageService.CreateImage(&compute.Image{
		SourceInstance: vm.Identification,
	})
	if err != nil {
		return "", err
	} else {
		return image.ID, nil
	}
}

func (cloug *Cloug) VmAddresses(vm *lobster.VirtualMachine) ([]*lobster.IpAddress, error) {
	if cloug.addressService == nil {
		return nil, fmt.Errorf("operation not supported")
	}
	apiAddresses, err := cloug.addressService.ListInstanceAddresses(vm.Identification)
	if err != nil {
		return nil, err
	}

	addresses := make([]*lobster.IpAddress, len(apiAddresses))
	for i, apiAddress := range apiAddresses {
		addresses[i] = &lobster.IpAddress{
			Ip:        apiAddress.IP,
			PrivateIp: apiAddress.PrivateIP,
			CanRdns:   apiAddress.CanDNS,
			Hostname:  apiAddress.Hostname,
		}
	}

	return addresses, nil
}

func (cloug *Cloug) VmAddAddress(vm *lobster.VirtualMachine) error {
	if cloug.addressService == nil {
		return fmt.Errorf("operation not supported")
	}
	return cloug.addressService.AddAddressToInstance(vm.Identification, new(compute.Address))
}

func (cloug *Cloug) VmRemoveAddress(vm *lobster.VirtualMachine, ip string, privateip string) error {
	if cloug.addressService == nil {
		return fmt.Errorf("operation not supported")
	}

	// list addresses to find ID matching IP
	apiAddresses, err := cloug.addressService.ListInstanceAddresses(vm.Identification)
	if err != nil {
		return err
	}

	for _, apiAddress := range apiAddresses {
		if apiAddress.IP == ip || apiAddress.PrivateIP == privateip {
			return cloug.addressService.RemoveAddressFromInstance(vm.Identification, apiAddress.ID)
		}
	}
	return fmt.Errorf("specified IP addresses not found on instance")
}

func (cloug *Cloug) VmSetRdns(vm *lobster.VirtualMachine, ip string, hostname string) error {
	if cloug.addressService == nil {
		return fmt.Errorf("operation not supported")
	}

	// list addresses to find ID matching IP
	apiAddresses, err := cloug.addressService.ListInstanceAddresses(vm.Identification)
	if err != nil {
		return err
	}

	for _, apiAddress := range apiAddresses {
		if apiAddress.IP == ip {
			return cloug.addressService.SetAddressHostname(apiAddress.ID, hostname)
		}
	}
	return fmt.Errorf("specified IP addresses not found on instance")
}

func (cloug *Cloug) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	info, err := cloug.VmInfo(vm)
	if err == nil {
		return info.BandwidthUsed
	} else {
		return 0
	}
}

func (cloug *Cloug) ImageFetch(url string, format string) (string, error) {
	if cloug.imageService == nil {
		return "", fmt.Errorf("operation not supported")
	}
	var regions []string
	if cloug.config.Region != "" {
		regions = []string{cloug.config.Region}
	}
	image, err := cloug.imageService.CreateImage(&compute.Image{
		Regions:   regions,
		SourceURL: url,
		Format:    format,
	})
	if err != nil {
		return "", err
	} else {
		return image.ID, nil
	}
}

func (cloug *Cloug) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	if cloug.imageService == nil {
		return nil, fmt.Errorf("operation not supported")
	}
	image, err := cloug.imageService.GetImage(imageIdentification)
	if err != nil {
		return nil, err
	}

	info := lobster.ImageInfo{
		Size:    image.Size,
		Details: image.Details,
	}

	if image.Status == compute.ImageAvailable {
		info.Status = lobster.ImageActive
	} else if image.Status == compute.ImagePending {
		info.Status = lobster.ImagePending
	}

	return &info, nil
}

func (cloug *Cloug) ImageDelete(imageIdentification string) error {
	if cloug.imageService == nil {
		return fmt.Errorf("operation not supported")
	}
	return cloug.imageService.DeleteImage(imageIdentification)
}

func (cloug *Cloug) ImageList() ([]*lobster.Image, error) {
	if cloug.imageService == nil {
		return nil, fmt.Errorf("operation not supported")
	}
	apiImages, err := cloug.imageService.ListImages()
	if err != nil {
		return nil, err
	}

	var images []*lobster.Image
	stringSliceContains := func(slice []string, search string) bool {
		for _, str := range slice {
			if str == search {
				return true
			}
		}
		return false
	}

	for _, apiImage := range apiImages {
		if len(apiImage.Regions) == 0 || cloug.config.Region == "" || stringSliceContains(apiImage.Regions, cloug.config.Region) {
			images = append(images, &lobster.Image{
				Name:           apiImage.Name,
				Identification: apiImage.ID,
			})
		}
	}
	return images, nil
}

func (cloug *Cloug) PlanList() ([]*lobster.Plan, error) {
	if cloug.flavorService == nil {
		return nil, fmt.Errorf("operation not supported")
	}
	flavors, err := cloug.flavorService.ListFlavors()
	if err != nil {
		return nil, err
	}
	plans := make([]*lobster.Plan, len(flavors))
	for i, flavor := range flavors {
		plans[i] = &lobster.Plan{
			Name:           flavor.Name,
			Ram:            flavor.MemoryMB,
			Cpu:            flavor.NumCores,
			Storage:        flavor.DiskGB,
			Bandwidth:      flavor.TransferGB,
			Identification: flavor.ID,
		}
	}
	return plans, nil
}
