package lobster

type VmInterface interface {
	// Creates a virtual machine with the given name, plan, and image.
	// vmIdentification != nil only if err == nil.
	VmCreate(name string, plan *Plan, imageIdentification string) (string, error)

	// Deletes the specified virtual machine.
	VmDelete(vmIdentification string) error

	VmInfo(vmIdentification string) (*VmInfo, error)

	VmStart(vmIdentification string) error
	VmStop(vmIdentification string) error
	VmReboot(vmIdentification string) error

	// On success, url is a link that we should redirect to.
	VmVnc(vmIdentification string) (string, error)
	CanVnc() bool

	// action is an element of VmInfo.Actions (although this is not guaranteed)
	VmAction(vmIdentification string, action string, value string) error

	VmRename(vmIdentification string, name string) error
	CanRename() bool

	VmReimage(vmIdentification string, imageIdentification string) error
	CanReimage() bool

	// returns the number of bytes transferred by the given VM since the last call
	// if this is the first call, then BandwidthAccounting must return zero
	BandwidthAccounting(vmIdentification string) int64

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
