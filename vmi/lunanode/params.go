package lunanode

const LNDYNAMIC_API_URL = "https://dynamic.lunanode.com/api/{CATEGORY}/{ACTION}/"

type APIGenericResponse struct {
	Success string `json:"success"`
	Error string `json:"error"`
}

// virtual machines

type APIVmCreateResponse struct {
	VmId string `json:"vm_id"`
}

type APIVmVncResponse struct {
	VncUrl string `json:"vnc_url"`
}

type APIVmInfoStruct struct {
	Ip string `json:"ip"`
	PrivateIp string `json:"privateip"`
	Status string `json:"status"`
	StatusColor string `json:"-"`
	Hostname string `json:"hostname"`
	BandwidthUsed string `json:"bandwidthUsedGB"`
	LoginDetails string `json:"login_details"`
	DiskSwap string `json:"diskswap"`
}

type APIVmInfoResponse struct {
	Info *APIVmInfoStruct `json:"info"`
}

// image

type APIImage struct {
	Id string `json:"image_id"`
	Name string `json:"name"`
	Status string `json:"status"`
	Size string `json:"size"`
}

type APIImageListResponse struct {
	Images []*APIImage `json:"images"`
}

type APIImageDetailsResponse struct {
	Image *APIImage `json:"details"`
}

type APIImageCreateResponse struct {
	Id string `json:"image_id"`
}

// volumes

type APIVolume struct {
	Id string `json:"id"`
	Name string `json:"name"`
	Size string `json:"size"`
	Region string `json:"region"`
	Status string `json:"status"`
}

type APIVolumeListResponse struct {
	Volumes []*APIVolume `json:"volumes"`
}

type APIVolumeInfoResponse struct {
	Volume *APIVolume `json:"volume"`
}

// plans

type APIPlan struct {
	Id string `json:"plan_id"`
	Name string `json:"name"`
	Vcpu string `json:"vcpu"`
	Price string `json:"price"`
	Ram string `json:"ram"`
	Storage string `json:"storage"`
	Bandwidth string `json:"bandwidth"`
}

type APIPlanListResponse struct {
	Plans []*APIPlan `json:"plans"`
}
