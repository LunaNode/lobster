package lobopenstack

import "lobster"
import "lobster/ipaddr"
import "errors"
import "log"
import "time"
import "github.com/LunaNode/gophercloud"
import "github.com/LunaNode/gophercloud/openstack"
import "github.com/LunaNode/gophercloud/pagination"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/flavors"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/servers"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/extensions/startstop"
import "github.com/LunaNode/gophercloud/openstack/compute/v2/extensions/floatingip"

type OpenStack struct {
	ComputeClient *gophercloud.ServiceClient
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
	return this
}

func (this *OpenStack) VmCreate(name string, plan *lobster.Plan, imageIdentification string) (string, error) {
	flavorOpts := flavors.ListOpts{
		MinDisk: plan.Storage,
		MinRAM: plan.Ram,
	}
	flavorPager := flavors.ListDetail(this.ComputeClient, flavorOpts)
	var matchFlavor *flavors.Flavor
	err := flavorPager.EachPage(func(page pagination.Page) (bool, error) {
		flavorList, err := flavors.ExtractFlavors(page)
		if err != nil {
			return false, err
		}

		for _, flavor := range flavorList {
			if flavor.Disk == plan.Storage && flavor.RAM == plan.Ram && flavor.VCPUs == plan.Cpu {
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

	opts := servers.CreateOpts{
		Name: name,
		ImageRef: imageIdentification,
		FlavorRef: matchFlavor.ID,
		Networks: []servers.Network{servers.Network{UUID: this.networkId}},
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

	return server.ID, nil
}

func (this *OpenStack) VmDelete(vmIdentification string) error {
	return servers.Delete(this.ComputeClient, vmIdentification).ExtractErr()
}

func (this *OpenStack) VmInfo(vmIdentification string) (*lobster.VmInfo, error) {
	server, err := servers.Get(this.ComputeClient, vmIdentification).Extract()
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
	}

	servers.ListAddresses(this.ComputeClient, vmIdentification).EachPage(func(page pagination.Page) (bool, error) {
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

func (this *OpenStack) VmStart(vmIdentification string) error {
	return startstop.Start(this.ComputeClient, vmIdentification).ExtractErr()
}

func (this *OpenStack) VmStop(vmIdentification string) error {
	return startstop.Stop(this.ComputeClient, vmIdentification).ExtractErr()
}

func (this *OpenStack) VmReboot(vmIdentification string) error {
	return servers.Reboot(this.ComputeClient, vmIdentification, servers.SoftReboot).ExtractErr()
}

func (this *OpenStack) VmVnc(vmIdentification string) (string, error) {
	return servers.Vnc(this.ComputeClient, vmIdentification, servers.NoVnc).Extract()
}

func (this *OpenStack) CanVnc() bool {
	return true
}

func (this *OpenStack) VmAction(vmIdentification string, action string, value string) error {
	return errors.New("operation not supported")
}

func (this *OpenStack) VmRename(vmIdentification string, name string) error {
	opts := servers.UpdateOpts{
		Name: name,
	}
	_, err := servers.Update(this.ComputeClient, vmIdentification, opts).Extract()
	return err
}

func (this *OpenStack) CanRename() bool {
	return true
}

func (this *OpenStack) VmReimage(vmIdentification string, imageIdentification string) error {
	opts := servers.RebuildOpts{
		ImageID: imageIdentification,
	}
	_, err := servers.Rebuild(this.ComputeClient, vmIdentification, opts).Extract()
	return err
}

func (this *OpenStack) CanReimage() bool {
	return true
}

func (this *OpenStack) BandwidthAccounting(vmIdentification string) int64 {
	return 0
}

func (this *OpenStack) CanImages() bool {
	return false
}

func (this *OpenStack) ImageFetch(url string, format string) (string, error) {
	return "", errors.New("operation not supported")
}

func (this *OpenStack) ImageInfo(imageIdentification string) (*lobster.ImageInfo, error) {
	return nil, errors.New("operation not supported")
}

func (this *OpenStack) ImageDelete(imageIdentification string) error {
	return errors.New("operation not supported")
}
