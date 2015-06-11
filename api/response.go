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
}

type Plan struct {
	Id int
	Name string
	Price int64
	Ram int
	Cpu int
	Storage int
	Bandwidth int
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

type PlanListResponse struct {
	Plans []*Plan `json:"plans"`
}
