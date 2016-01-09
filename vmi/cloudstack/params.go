package cloudstack

type APIErrorResponse struct {
	ErrorCode int    `json:"errorcode"`
	ErrorText string `json:"errortext"`
}

type APIIDResponse struct {
	ID string `json:"id"`
}

type APIServiceOffering struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CPUNumber int    `json:"cpunumber"`
	Memory    int    `json:"memory"`
}

type APIListServiceOfferingsResponse struct {
	ServiceOfferings []APIServiceOffering `json:"serviceoffering"`
}

type APIDiskOffering struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DiskSize     int    `json:"disksize"`
	IsCustomized bool   `json:"iscustomized"`
}

type APIListDiskOfferingsResponse struct {
	DiskOfferings []APIDiskOffering `json:"diskoffering"`
}

type APINic struct {
	Addr string `json:"ipaddress"`
}

type APIDeployVirtualMachineResponse struct {
	ID    string `json:"id"`
	JobID string `json:"jobid"`
}

type APIDeployVirtualMachineResult struct {
	Password string `json:"password"`
}

type APIVirtualMachine struct {
	State string   `json:"state"`
	Nics  []APINic `json:"nic"`
}

type APIListVirtualMachinesResponse struct {
	VirtualMachines []APIVirtualMachine `json:"virtualmachine"`
}
