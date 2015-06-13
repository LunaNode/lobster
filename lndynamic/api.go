package lndynamic

import "bytes"
import "crypto/sha512"
import "crypto/hmac"
import "crypto/rand"
import "encoding/json"
import "encoding/hex"
import "errors"
import "fmt"
import "io/ioutil"
import "net/url"
import "net/http"
import "strconv"
import "strings"
import "time"

type API struct {
	ApiId string
	ApiKey string
	ApiPartialKey string
}

func MakeAPI(id string, key string) (*API, error) {
	if len(id) != 16 {
		return nil, errors.New(fmt.Sprintf("API ID length must be 16, but parameter has length %d", len(id)))
	} else if len(key) != 128 {
		return nil, errors.New(fmt.Sprintf("API key length must be 128, but parameter has length %d", len(key)))
	}

	this := new(API)
	this.ApiId = id
	this.ApiKey = key
	this.ApiPartialKey = key[:64]
	return this, nil
}

func (this *API) request(category string, action string, params map[string]string, target interface{}) error {
	// construct URL
	targetUrl := LNDYNAMIC_API_URL
	targetUrl = strings.Replace(targetUrl, "{CATEGORY}", category, -1)
	targetUrl = strings.Replace(targetUrl, "{ACTION}", action, -1)

	// get raw parameters string
	if params == nil {
		params = make(map[string]string)
	}
	params["api_id"] = this.ApiId
	params["api_partialkey"] = this.ApiPartialKey
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}

	// HMAC signature with nonce
	nonce := fmt.Sprintf("%d", time.Now().Unix())
	handler := fmt.Sprintf("%s/%s/", category, action)
	hashTarget := fmt.Sprintf("%s|%s|%s", handler, string(rawParams), nonce)
	hasher := hmac.New(sha512.New, []byte(this.ApiKey))
	_, err = hasher.Write([]byte(hashTarget))
	if err != nil {
		return err
	}
	signature := hex.EncodeToString(hasher.Sum(nil))

	// perform request
	values := url.Values{}
	values.Set("handler", handler)
	values.Set("req", string(rawParams))
	values.Set("signature", signature)
	values.Set("nonce", nonce)
	byteBuffer := new(bytes.Buffer)
	byteBuffer.Write([]byte(values.Encode()))
	response, err := http.Post(targetUrl, "application/x-www-form-urlencoded", byteBuffer)
	if err != nil {
		return err
	}
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	response.Body.Close()

	// decode JSON
	// we first decode into generic response for error checking; then into specific response to return
	var genericResponse APIGenericResponse
	err = json.Unmarshal(responseBytes, &genericResponse)

	if err != nil {
		return err
	} else if genericResponse.Success != "yes" {
		if genericResponse.Error != "" {
			return errors.New(genericResponse.Error)
		} else {
			return errors.New("backend call failed for unknown reason")
		}
	}

	if target != nil {
		err = json.Unmarshal(responseBytes, target)
		if err != nil {
			return err
		}
	}

	return nil
}

func (this *API) uid() string {
	bytes := make([]byte, 12)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(bytes)
}

// virtual machines

func (this *API) VmCreateVolume(region string, hostname string, planIdentification int, volumeIdentification int) (int, error) {
	params := make(map[string]string)
	params["hostname"] = hostname
	params["region"] = region
	params["plan_id"] = fmt.Sprintf("%d", planIdentification)
	params["volume_id"] = fmt.Sprintf("%d", volumeIdentification)
	var response APIVmCreateResponse
	err := this.request("vm", "create", params, &response)
	if err != nil {
		return 0, err
	} else {
		vmId, err := strconv.ParseInt(response.VmId, 10, 32)
		if err != nil {
			return 0, err
		} else {
			return int(vmId), nil
		}
	}
}
func (this *API) VmCreateImage(region string, hostname string, planIdentification int, imageIdentification int) (int, error) {
	params := make(map[string]string)
	params["hostname"] = hostname
	params["region"] = region
	params["plan_id"] = fmt.Sprintf("%d", planIdentification)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	var response APIVmCreateResponse
	err := this.request("vm", "create", params, &response)
	if err != nil {
		return 0, err
	} else {
		vmId, err := strconv.ParseInt(response.VmId, 10, 32)
		if err != nil {
			return 0, err
		} else {
			return int(vmId), nil
		}
	}
}

func (this *API) vmAction(vmIdentification int, action string, params map[string]string) error {
	if params == nil {
		params = make(map[string]string)
	}
	params["vm_id"] = fmt.Sprintf("%d", vmIdentification)
	return this.request("vm", action, params, nil)
}

func (this *API) VmStart(vmIdentification int) error {
	return this.vmAction(vmIdentification, "start", nil)
}

func (this *API) VmStop(vmIdentification int) error {
	return this.vmAction(vmIdentification, "stop", nil)
}

func (this *API) VmReboot(vmIdentification int) error {
	return this.vmAction(vmIdentification, "reboot", nil)
}

func (this *API) VmDelete(vmIdentification int) error {
	return this.vmAction(vmIdentification, "delete", nil)
}

func (this *API) VmDiskSwap(vmIdentification int) error {
	return this.vmAction(vmIdentification, "diskswap", nil)
}

func (this *API) VmReimage(vmIdentification int, imageIdentification int) error {
	params := make(map[string]string)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	return this.vmAction(vmIdentification, "reimage", params)
}

func (this *API) VmVnc(vmIdentification int) (string, error) {
	params := make(map[string]string)
	params["vm_id"] = fmt.Sprintf("%d", vmIdentification)
	var response APIVmVncResponse
	err := this.request("vm", "vnc", params, &response)
	if err != nil {
		return "", err
	} else {
		return response.VncUrl, nil
	}
}

func (this *API) VmInfo(vmIdentification int) (*APIVmInfoStruct, error) {
	params := make(map[string]string)
	params["vm_id"] = fmt.Sprintf("%d", vmIdentification)
	var response APIVmInfoResponse
	err := this.request("vm", "info", params, &response)
	if err != nil {
		return nil, err
	} else {
		// fix status
		statusParts := strings.Split(response.Info.Status, "b&gt;")
		if len(statusParts) >= 2 {
			response.Info.StatusColor = strings.Split(strings.Split(response.Info.Status, "color=&quot;")[1], "&quot;")[0]
			response.Info.Status = strings.Split(statusParts[1], "&lt;")[0]
		} else {
			response.Info.Status = "Unknown"
			response.Info.StatusColor = "blue"
		}
		return response.Info, nil
	}
}

func (this *API) VmSnapshot(vmIdentification int, region string) (int, error) {
	// create snapshot with random label
	imageLabel := this.uid()
	params := make(map[string]string)
	params["vm_id"] = fmt.Sprintf("%d", vmIdentification)
	params["name"] = imageLabel
	var response APIGenericResponse
	err := this.request("vm", "snapshot", params, &response)
	if err != nil {
		return 0, err
	}

	// find the image ID based on label
	params = make(map[string]string)
	params["region"] = region
	var listResponse APIImageListResponse
	err = this.request("image", "list", params, &listResponse)
	if err != nil {
		return 0, err
	}

	for _, image := range listResponse.Images {
		if strings.Contains(image.Name, imageLabel) {
			imageId, err := strconv.ParseInt(image.Id, 10, 32)
			if err != nil {
				return 0, err
			} else {
				return int(imageId), nil
			}
		}
	}

	return 0, errors.New("backend reported successful snapshot creation, but not found in list")
}

// images

func (this *API) ImageFetch(region string, location string, format string, virtio bool) (int, error) {
	// create an image with random label
	imageLabel := this.uid()
	params := make(map[string]string)
	params["region"] = region
	params["name"] = imageLabel
	params["location"] = location
	params["format"] = format
	if virtio {
		params["virtio"] = "yes"
	}
	var response APIGenericResponse
	err := this.request("image", "fetch", params, &response)
	if err != nil {
		return 0, err
	}

	// find the image ID based on label
	params = make(map[string]string)
	params["region"] = region
	var listResponse APIImageListResponse
	err = this.request("image", "list", params, &listResponse)
	if err != nil {
		return 0, err
	}

	for _, image := range listResponse.Images {
		if strings.Contains(image.Name, imageLabel) {
			imageId, err := strconv.ParseInt(image.Id, 10, 32)
			if err != nil {
				return 0, err
			} else {
				return int(imageId), nil
			}
		}
	}

	return 0, errors.New("backend reported successful image creation, but not found in list")
}

func (this *API) ImageDetails(imageIdentification int) (*APIImage, error) {
	params := make(map[string]string)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	var response APIImageDetailsResponse
	err := this.request("image", "details", params, &response)
	if err != nil {
		return nil, err
	} else {
		return response.Image, nil
	}
}

func (this *API) ImageDelete(imageIdentification int) error {
	params := make(map[string]string)
	params["image_id"] = fmt.Sprintf("%d", imageIdentification)
	var response APIGenericResponse
	return this.request("image", "delete", params, &response)
}

// volumes

// Create a volume with the given size in gigabytes and image identification.
// If timeout is greater than zero, we will wait for the volume to become ready, or return error if timeout is exceeded.
// Otherwise, we return immediately without error.
func (this *API) VolumeCreate(region string, size int, imageIdentification int, timeout time.Duration) (int, error) {
	// create a volume with random label
	volumeLabel := this.uid()
	params := make(map[string]string)
	params["region"] = region
	params["label"] = volumeLabel
	params["size"] = fmt.Sprintf("%d", size)
	params["image"] = fmt.Sprintf("%d", imageIdentification)
	var response APIGenericResponse
	err := this.request("volume", "create", params, &response)
	if err != nil {
		return 0, err
	}

	// find the volume ID based on label
	params = make(map[string]string)
	params["region"] = region
	var listResponse APIVolumeListResponse
	err = this.request("volume", "list", params, &listResponse)
	if err != nil {
		return 0, err
	}

	var volumeId int
	for _, volume := range listResponse.Volumes {
		if strings.Contains(volume.Name, volumeLabel) {
			volumeId64, err := strconv.ParseInt(volume.Id, 10, 32)
			if err != nil {
				return 0, err
			} else {
				volumeId = int(volumeId64)
			}
		}
	}

	if volumeId == 0 {
		return 0, errors.New("API reported successful volume creation, but not found in list")
	} else if timeout <= 0 {
		return volumeId, nil
	}

	// wait for volume to create
	startTime := time.Now()
	for time.Now().Before(startTime.Add(timeout)) {
		params = make(map[string]string)
		params["region"] = region
		params["volume_id"] = fmt.Sprintf("%d", volumeId)
		var infoResponse APIVolumeInfoResponse
		err = this.request("volume", "info", params, &infoResponse)
		if err != nil {
			break
		} else if infoResponse.Volume.Status == "available" {
			return volumeId, nil
		} else {
			time.Sleep(5 * time.Second)
		}
	}

	return 0, errors.New(fmt.Sprintf("volume creation timeout exceeded (%d seconds)", timeout.Seconds()))
}

func (this *API) VolumeDelete(region string, volumeIdentification int) error {
	params := make(map[string]string)
	params["region"] = region
	params["volume_id"] = fmt.Sprintf("%d", volumeIdentification)
	var response APIGenericResponse
	return this.request("volume", "delete", params, &response)
}

// plans

func (this *API) PlanList() ([]*APIPlan, error) {
	var listResponse APIPlanListResponse
	err := this.request("plan", "list", nil, &listResponse)
	return listResponse.Plans, err
}
