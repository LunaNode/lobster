package api

import "bytes"
import "crypto/hmac"
import "crypto/sha512"
import "encoding/hex"
import "encoding/json"
import "errors"
import "fmt"
import "io"
import "io/ioutil"
import "net/http"
import "time"

type Client struct {
	Url string
	ApiId string
	ApiKey string
}

func (this *Client) request(method string, path string, requestObj interface{}, responseObj interface{}) error {
	var requestBytes []byte
	var body io.Reader
	if requestObj != nil {
		var err error
		requestBytes, err = json.Marshal(requestObj)
		if err != nil {
			return err
		}
		body = bytes.NewBuffer(requestBytes)
	}

	// signature is hmac_{apikey}(path|nonce|request)
	nonce := time.Now().UnixNano()
	mac := hmac.New(sha512.New, []byte(this.ApiKey))
	toSign := fmt.Sprintf("%s|%d|%s", path, nonce, string(requestBytes))
	mac.Write([]byte(toSign))
	signature := hex.EncodeToString(mac.Sum(nil))

	request, err := http.NewRequest(method, this.Url + path, body)
	if err != nil {
		return err
	}
	request.Header.Add("Authorization", fmt.Sprintf("lobster %s:%s:%d:%s", this.ApiId, this.ApiKey[:64], nonce, signature))

	c := new(http.Client)
	response, err := c.Do(request)
	if err != nil {
		return err
	}
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	} else if response.StatusCode < 200 || response.StatusCode > 204 {
		return errors.New(string(responseBytes))
	}

	if responseObj != nil {
		err = json.Unmarshal(responseBytes, responseObj)
		if err != nil {
			return err
		}
	}

	return nil
}

func (this *Client) VmList() ([]*VirtualMachine, error) {
	var response VMListResponse
	err := this.request("GET", "vms", nil, &response)
	if err != nil {
		return nil, err
	} else {
		return response.VirtualMachines, nil
	}
}

func (this *Client) VmCreate(name string, planId int, imageId int) (int, error) {
	request := VMCreateRequest{
		Name: name,
		PlanId: planId,
		ImageId: imageId,
	}
	var response VMCreateResponse
	err := this.request("POST", "vms", request, &response)
	if err != nil {
		return 0, err
	} else {
		return response.Id, nil
	}
}

func (this *Client) VmInfo(vmId int) (*VMInfoResponse, error) {
	var response VMInfoResponse
	err := this.request("GET", fmt.Sprintf("vm/%d", vmId), nil, &response)
	if err != nil {
		return nil, err
	} else {
		return &response, nil
	}
}

func (this *Client) VmAction(vmId int, action string, value string) error {
	request := VMActionRequest{
		Action: action,
		Value : value,
	}
	return this.request("POST", fmt.Sprintf("vm/%d/action", vmId), request, nil)
}

func (this *Client) VmVnc(vmId int) (string, error) {
	request := VMActionRequest{
		Action: "vnc",
	}
	var response VMVncResponse
	err := this.request("POST", fmt.Sprintf("vm/%d/action", vmId), request, &response)
	if err != nil {
		return "", err
	} else {
		return response.Url, nil
	}
}

func (this *Client) VmReimage(vmId int, imageId int) error {
	request := VMReimageRequest{
		ImageId: imageId,
	}
	return this.request("POST", fmt.Sprintf("vm/%d/reimage", vmId), request, nil)
}

func (this *Client) VmDelete(vmId int) error {
	return this.request("DELETE", fmt.Sprintf("vm/%d", vmId), nil, nil)
}

func (this *Client) PlanList() ([]*Plan, error) {
	var response PlanListResponse
	err := this.request("GET", "plans", nil, &response)
	if err != nil {
		return nil, err
	} else {
		return response.Plans, nil
	}
}
