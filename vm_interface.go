package lobster

type VmInterface interface {
	// Creates a virtual machine with the given name and plan (specified in vm object), and image.
	// Returns vmIdentification string and optional error.
	// Should return vmIdentification != "" only if err == nil.
	VmCreate(vm *VirtualMachine, imageIdentification string) (string, error)

	// Deletes the specified virtual machine.
	VmDelete(vm *VirtualMachine) error

	VmInfo(vm *VirtualMachine) (*VmInfo, error)

	VmStart(vm *VirtualMachine) error
	VmStop(vm *VirtualMachine) error
	VmReboot(vm *VirtualMachine) error

	// On success, url is a link that we should redirect to.
	VmVnc(vm *VirtualMachine) (string, error)
	CanVnc() bool

	// action is an element of VmInfo.Actions (although this is not guaranteed)
	VmAction(vm *VirtualMachine, action string, value string) error

	VmRename(vm *VirtualMachine, name string) error
	CanRename() bool

	VmReimage(vm *VirtualMachine, imageIdentification string) error
	CanReimage() bool

	CanAddresses() bool
	VmAddresses(vm *VirtualMachine) ([]*IpAddress, error)
	VmAddAddress(vm *VirtualMachine) error
	VmRemoveAddress(vm *VirtualMachine, ip string, privateip string) error
	VmSetRdns(vm *VirtualMachine, ip string, hostname string) error

	// returns the number of bytes transferred by the given VM since the last call
	// if this is the first call, then BandwidthAccounting must return zero
	BandwidthAccounting(vm *VirtualMachine) int64

	// Whether ImageFetch, ImageDetails, and ImageDelete are supported
	CanImages() bool

	// Download an image from an external URL.
	// Format is currently either 'template' or 'iso' in the form, although user may provide arbitrary format string.
	ImageFetch(url string, format string) (string, error)

	ImageInfo(imageIdentification string) (*ImageInfo, error)
	ImageDelete(imageIdentification string) error
}

// Note: before calling the VmInterface, we will make sure that the user actually owns the virtual machine
// with the given identification, and same for images. However, VmInterface is responsible for checking any
// other input, e.g. action strings, action parameters, image formats, image URLs.
