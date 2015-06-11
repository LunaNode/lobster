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
