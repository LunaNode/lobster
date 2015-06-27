package api

type VMCreateRequest struct {
	Name string `json:"name"`
	PlanId int `json:"plan_id"`
	ImageId int `json:"image_id"`
}

type VMActionRequest struct {
	Action string `json:"action"`
	Value string `json:"value"`
}

type VMReimageRequest struct {
	ImageId int `json:"image_id"`
}

type VMResizeRequest struct {
	PlanId int `json:"plan_id"`
}

type VMAddressRemoveRequest struct {
	Ip string `json:"ip"`
	PrivateIp string `json:"private_ip"`
}

type VMAddressRdnsRequest struct {
	Hostname string `json:"hostname"`
}

type ImageFetchRequest struct {
	Region string `json:"region"`
	Name string `json:"name"`
	Url string `json:"url"`
	Format string `json:"format"`
}
