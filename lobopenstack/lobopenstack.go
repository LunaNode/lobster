package lobopenstack

import "github.com/LunaNode/lobster"
import "github.com/LunaNode/lobster/ipaddr"
import "github.com/LunaNode/lobster/utils"

import "github.com/LunaNode/gophercloud"
import "github.com/LunaNode/gophercloud/openstack"
import "github.com/LunaNode/gophercloud/pagination"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/flavors"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/servers"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/extensions/startstop"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/extensions/floatingip"
import "github.com/LunaNode/gophercloud/openstack/image/v1/image"

import "errors"
import "log"
import "strconv"
import "strings"
import "time"

type OpenStack struct {
	ComputeClient *gophercloud.ServiceClient
	ImageClient *gophercloud.ServiceClient
	networkId string
}

func MakeOpenStack(identityEndpoint string, username string, password string, tenantName string, networkId string) *OpenStack {
	this := new(OpenStack)
	this.networkId = networkId
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: identityEndpoint,
		Username: username,
		Password: password,
		TenantName: tenantName,
	}
	provider, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		panic(err)
	}
	this.ComputeClient, err = openstack.NewComputeV2(provider, gophercloud.EndpointOpts{})
	if err != nil {
		panic(err)
	}
	this.ImageClient, err = openstack.NewImageV1(provider, gophercloud.EndpointOpts{})
	if err != nil {
		panic(err)
	}
	return this
}

func (this *OpenStack) VmCreate(vm *lobster.VirtualMachine, imageIdentification string) (string, error) {
	flavorOpts := flavors.ListOpts{
		MinDisk: vm.Plan.Storage,
		MinRAM: vm.Plan.Ram,
	}
	flavorPager := flavors.ListDetail(this.ComputeClient, flavorOpts)
	var matchFlavor *flavors.Flavor
	err := flavorPager.EachPage(func(page pagination.Page) (bool, error) {
		flavorList, err := flavors.ExtractFlavors(page)
		if err != nil {
			return false, err
		}

		for _, flavor := range flavorList {
			if flavor.Disk == vm.Plan.Storage && flavor.RAM == vm.Plan.Ram && flavor.VCPUs == vm.Plan.Cpu {
				matchFlavor = &flavor
				return false, nil
			}
		}
		return true, nil
	})
	if err != nil {
		return "", err
	} else if matchFlavor == nil {
		return "", errors.New("plan not available in this region")
	}

	password := utils.Uid(16)
	opts := servers.CreateOpts{
		Name: vm.Name,
		ImageRef: imageIdentification,
		FlavorRef: matchFlavor.ID,
		Networks: []servers.Network{servers.Network{UUID: this.networkId}},
		AdminPass: password,
		UserData: []byte("#cloud-config\npassword: " + password + "\nchpasswd: { expire: False }\nssh_pwauth: True\n"),
	}
	createResult := servers.Create(this.ComputeClient, opts)
	server, err := createResult.Extract()
	if err != nil {
		return "", err
	}

	// try to associate floating IP with this VM
	// do asynchronously since it might fail until network port is created
	go func() {
		for try := 0; try < 6; try++ {
			time.Sleep(4 * time.Second)

			// find a free floating IP
			var freeFloatingIp *floatingip.FloatingIP
			err := floatingip.List(this.ComputeClient).EachPage(func(page pagination.Page) (bool, error) {
				floatingIps, err := floatingip.ExtractFloatingIPs(page)
				if err != nil {
					return false, err
				}

				for _, floatingIp := range floatingIps {
					if floatingIp.InstanceID == "" {
						freeFloatingIp = &floatingIp
						return false, nil
					}
				}
				return true, nil
			})
			if err != nil {
				log.Printf("OpenStack: error while looking for free floating IP: %s", err.Error())
				continue
			} else if freeFloatingIp == nil {
				log.Printf("OpenStack: Did not find free floating IP!")
				continue
			}

			// associate it
			err = floatingip.Associate(this.ComputeClient, server.ID, freeFloatingIp.IP).ExtractErr()
			if err == nil {
				break
			} else {
				log.Printf("OpenStack: error while associating floating IP: %s", err.Error())
			}
		}
	}()

	vm.SetMetadata("password", password)
	return server.ID, nil
}

func (this *OpenStack) VmDelete(vm *lobster.VirtualMachine) error {
	return servers.Delete(this.ComputeClient, vm.Identification).ExtractErr()
}

func (this *OpenStack) VmInfo(vm *lobster.VirtualMachine) (*lobster.VmInfo, error) {
	server, err := servers.Get(this.ComputeClient, vm.Identification).Extract()
	if err != nil {
		return nil, err
	}

	status := server.Status
	if status == "ACTIVE" {
		status = "Online"
	} else if status == "SHUTOFF" {
		status = "Offline"
	}

	info := lobster.VmInfo{
		Status: status,
		Hostname: server.Name,
		LoginDetails: "password: " + vm.Metadata("password", "unknown"),
	}

	servers.ListAddresses(this.ComputeClient, vm.Identification).EachPage(func(page pagination.Page) (bool, error) {
		addresses, err := servers.ExtractAddresses(page)
		if err != nil {
			return false, err
		}

		for _, networkAddresses := range addresses {
			for _, addr := range networkAddresses {
				if ipaddr.IsPrivate(addr.Address) {
					info.PrivateIp = addr.Address
				} else {
					info.Ip = addr.Address
				}
			}
		}
		return true, nil
	})

	return &info, nil
}

func (this *OpenStack) VmStart(vm *lobster.VirtualMachine) error {
	return startstop.Start(this.ComputeClient, vm.Identification).ExtractErr()
}

func (this *OpenStack) VmStop(vm *lobster.VirtualMachine) error {
	return startstop.Stop(this.ComputeClient, vm.Identification).ExtractErr()
}

func (this *OpenStack) VmReboot(vm *lobster.VirtualMachine) error {
	return servers.Reboot(this.ComputeClient, vm.Identification, servers.SoftReboot).ExtractErr()
}

func (this *OpenStack) VmVnc(vm *lobster.VirtualMachine) (string, error) {
	return servers.Vnc(this.ComputeClient, vm.Identification, servers.NoVnc).Extract()
}

func (this *OpenStack) VmAction(vm *lobster.VirtualMachine, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *OpenStack) VmRename(vm *lobster.VirtualMachine, name string) error {
	opts := servers.UpdateOpts{
		Name: name,
	}
	_, err := servers.Update(this.ComputeClient, vm.Identification, opts).Extract()
	return err
}

func (this *OpenStack) VmReimage(vm *lobster.VirtualMachine, imageIdentification string) error {
	opts := servers.RebuildOpts{
		ImageID: imageIdentification,
	}
	_, err := servers.Rebuild(this.ComputeClient, vm.Identification, opts).Extract()
	return err
}

func (this *OpenStack) VmSnapshot(vm *lobster.VirtualMachine) (string, error) {
	opts := servers.CreateImageOpts{
		Name: utils.Uid(16),
	}
	return servers.CreateImage(this.ComputeClient, vm.Identification, opts).ExtractImageID()
}

func (this *OpenStack) BandwidthAccounting(vm *lobster.VirtualMachine) int64 {
	return 0
}

func (this *OpenStack) ImageFetch(url string, format string) (string, error) {
	opts := image.CreateOpts{
		Name: "lobster",
		ContainerFormat: "bare",
		DiskFormat: format,
		CopyFrom: url,
	}
	createResult := image.Create(this.ImageClient, opts)
	image, err := createResult.Extract()
	if err != nil {
		return "", err
	} else {
		return image.ID, nil
	}
}

func (this *OpenStack) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	osImage, err := image.Get(this.ImageClient, imageIdentification).Extract()
	if err != nil {
		return nil, err
	}

	image := new(lobster.ImageInfo)
	image.Size, _ = strconv.ParseInt(osImage.Size, 10, 64)
	if osImage.Status == "active" {
		image.Status = lobster.ImageActive
	} else if osImage.Status == "error" || osImage.Status == "killed" {
		image.Status = lobster.ImageError
	} else {
		image.Status = lobster.ImagePending
	}
	return image, nil
}

func (this *OpenStack) ImageDelete(imageIdentification string) error {
	err := image.Delete(this.ImageClient, imageIdentification).ExtractErr()
	if err != nil && !strings.Contains(err.Error(), "Image with identifier " + imageIdentification + " not found") {
		return err
	} else {
		return nil
	}
}
