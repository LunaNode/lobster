package main

import "lobster"
import "lobster/lndynamic"
import "lobster/solusvm"
import "lobster/lobopenstack"
import "os"

func main() {
	cfgPath := "lobster.cfg"
	if len(os.Args) >= 2 {
		cfgPath = os.Args[1]
	}
	app := lobster.MakeLobster(cfgPath)
	app.Init()

	// virtual machine interfaces
	lndToronto := lndynamic.MakeLNDynamic("toronto", "apiId", "apiKey")
	app.RegisterVmInterface("Toronto", lndToronto)

	solusKVM := &solusvm.SolusVM{
		VirtType: "kvm",
		NodeGroup: "1",
		Api: &solusvm.API{
			Url: "https://167.114.196.224:5656/api/admin/command.php",
			ApiId: "RZGxoFGpgpGiudvIatxsg0a4tEaH1mnQTM5nhjux",
			ApiKey: "PCSTZqTiObpOci9GN9cCkqi43chamBx36gLUwu3b",
			Insecure: true,
		},
	}
	app.RegisterVmInterface("KVM", soluskvm)

	solusVZ := &solusvm.SolusVM{
		VirtType: "openvz",
		NodeGroup: "1",
		Lobster: app,
		Api: &solusvm.API{
			Url: "https://167.114.196.224:5656/api/admin/command.php",
			ApiId: "RZGxoFGpgpGiudvIatxsg0a4tEaH1mnQTM5nhjux",
			ApiKey: "PCSTZqTiObpOci9GN9cCkqi43chamBx36gLUwu3b",
			Insecure: true,
		},
	}
	app.RegisterVmInterface("OpenVZ", solusVZ)

	openstack := lobopenstack.MakeOpenStack("http://controller:35357/v2.0", "username", "password", "tenantname", "internal-network-uuid")
	app.RegisterVmInterface("OpenStack", openstack)

	// payment interfaces
	paypal := lobster.MakePaypalPayment(app, "payments@example.com", "https://example.com/")
	app.RegisterPaymentInterface("Paypal", paypal)
	coinbase := lobster.MakeCoinbasePayment(app, "callback secret", "apiId", "apiKey")
	app.RegisterPaymentInterface("Bitcoin", coinbase)

	app.Run()
}
