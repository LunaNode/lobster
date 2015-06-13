package api

type VirtualMachine struct {
	Id int `json:"id"`
	PlanId int `json:"plan_id"`

	Region string `json:"region"`
	Name string `json:"name"`
	Status string `json:"status"`
	TaskPending bool `json:"task_pending"`
	ExternalIP string `json:"external_ip"`
	PrivateIP string `json:"private_ip"`
	CreatedTime int64 `json:"created_time"`
}

type VirtualMachineAction struct {
	Action string `json:"action"`
	Name string `json:"name"`
	Options map[string]string `json:"options"`
	Description string `json:"description"`
	Dangerous bool `json:"dangerous"`
}

type VirtualMachineDetails struct {
	Ip string `json:"ip"`
	PrivateIp string `json:"private_ip"`
	Status string `json:"status"`
	Hostname string `json:"hostname"`
	BandwidthUsed int64 `json:"bandwidth_used"`
	LoginDetails string `json:"login_details"`
	Details map[string]string `json:"details"`
	Actions []*VirtualMachineAction `json:"actions"`
	CanVnc bool `json:"can_vnc"`
	CanReimage bool `json:"can_reimage"`
	CanSnapshot bool `json:"can_snapshot"`
	CanAddresses bool `json:"can_addresses"`
}

type IpAddress struct {
	Ip string `json:"ip"`
	PrivateIp string `json:"private_ip"`
	CanRdns bool `json:"can_rdns"`
	Hostname string `json:"hostname"`
}

type Image struct {
	Id int `json:"id"`
	Region string `json:"region"`
	Name string `json:"name"`
	Status string `json:"status"`
}

type ImageDetails struct {
	Size int64 `json:"size"`
	Status string `json:"status"`
	Details map[string]string `json:"details"`
}

type Plan struct {
	Id int `json:"id"`
	Name string `json:"name"`
	Price int64 `json:"price"`
	Ram int `json:"ram"`
	Cpu int `json:"cpu"`
	Storage int `json:"storage"`
	Bandwidth int `json:"bandwidth"`
}

// responses

type VMListResponse struct {
	VirtualMachines []*VirtualMachine `json:"vms"`
}

type VMCreateResponse struct {
	Id int `json:"id"`
}

type VMInfoResponse struct {
	VirtualMachine *VirtualMachine `json:"vm"`
	Details *VirtualMachineDetails `json:"details"`
}

type VMVncResponse struct {
	Url string `json:"url"`
}

type VMSnapshotResponse struct {
	Id int `json:"id"`
}

type VMAddressesResponse struct {
	Addresses []*IpAddress `json:"addresses"`
}

type ImageListResponse struct {
	Images []*Image `json:"images"`
}

type ImageFetchResponse struct {
	Id int `json:"id"`
}

type ImageInfoResponse struct {
	Image *Image `json:"image"`
	Details *ImageDetails `json:"details"`
}

type PlanListResponse struct {
	Plans []*Plan `json:"plans"`
}
