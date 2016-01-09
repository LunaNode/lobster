package cloudstack

import "crypto/sha1"
import "crypto/hmac"
import "encoding/json"
import "encoding/base64"
import "errors"
import "fmt"
import "io/ioutil"
import "net/url"
import "net/http"
import "sort"
import "strings"

type API struct {
	TargetURL string
	ZoneID string
	APIKey string
	SecretKey string

	NetworkID string
}

func (api *API) request(command string, requestParams map[string]string, target interface{}) error {
	// add default params
	params := make(map[string]string)
	params["command"] = command
	params["apiKey"] = api.APIKey
	params["response"] = "json"
	params["zoneid"] = api.ZoneID
	for k, v := range requestParams {
		params[k] = v
	}

	// get sorted slice of param keys
	sortedParamKeys := make([]string, len(params))
	i := 0
	for k := range params {
		sortedParamKeys[i] = k
		i++
	}
	sort.Sort(sort.StringSlice(sortedParamKeys))

	// build URL / command string
	var commandStrParts []string
	requestURL, err := url.Parse(api.TargetURL)
	requestQuery := make(url.Values)
	if err != nil {
		return err
	}
	for _, k := range sortedParamKeys {
		requestQuery.Set(k, params[k])
		commandStrParts = append(commandStrParts, fmt.Sprintf("%s=%s", url.QueryEscape(k), url.QueryEscape(params[k])))
	}
	commandStr := strings.ToLower(strings.Join(commandStrParts, "&"))

	// HMAC signature
	hasher := hmac.New(sha1.New, []byte(api.SecretKey))
	_, err = hasher.Write([]byte(commandStr))
	if err != nil {
		return err
	}
	signature := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

	requestQuery.Set("signature", signature)
	requestURL.RawQuery = requestQuery.Encode()

	// perform request
	response, err := http.Get(requestURL.String())
	if err != nil {
		return err
	}
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	response.Body.Close()

	// decode JSON
	responseMap := make(map[string]interface{})
	err = json.Unmarshal(responseBytes, &responseMap)
	var objectKey string
	var objectValue []byte
	for k, v := range responseMap {
		objectKey = k
		objectValue, err = json.Marshal(v)
		if err != nil {
			return err
		}
	}

	if objectKey == "errorresponse" || strings.Contains(string(objectValue), "errortext") {
		var errorResponse APIErrorResponse
		err = json.Unmarshal(objectValue, &errorResponse)
		if err != nil {
			return err
		} else {
			return errors.New(errorResponse.ErrorText)
		}
	} else if target != nil {
		err = json.Unmarshal(objectValue, target)
		if err != nil {
			return err
		}
	}

	return nil
}

func (api *API) ListServiceOfferings() ([]APIServiceOffering, error) {
	var response APIListServiceOfferingsResponse
	err := api.request("listServiceOfferings", nil, &response)
	if err != nil {
		return nil, err
	} else {
		return response.ServiceOfferings, nil
	}
}

func (api *API) ListDiskOfferings() ([]APIDiskOffering, error) {
	var response APIListDiskOfferingsResponse
	err := api.request("listDiskOfferings", nil, &response)
	if err != nil {
		return nil, err
	} else {
		return response.DiskOfferings, nil
	}
}

func (api *API) DeployVirtualMachine(serviceOfferingID string, diskOfferingID string, templateID string) (string, string, error) {
	params := map[string]string{
		"serviceofferingid": serviceOfferingID,
		"diskofferingid": diskOfferingID,
		"templateid": templateID,
		"networkids": api.NetworkID,
	}
	var response APIDeployVirtualMachineResponse
	err := api.request("deployVirtualMachine", params, &response)
	if err != nil {
		return "", "", err
	} else {
		return response.ID, response.JobID, nil
	}
}

func (api *API) QueryDeployJob(jobid string) (*APIDeployVirtualMachineResult, error) {
	type JobResult struct {
		VirtualMachine *APIDeployVirtualMachineResult `json:"virtualmachine"`
	}
	type Response struct {
		Status int `json:"jobstatus"`
		Result JobResult `json:"jobresult"`
	}
	var response Response
	err := api.request("queryAsyncJobResult", map[string]string{"jobid": jobid}, &response)
	if err != nil {
		return nil, err
	} else if response.Status == 0 {
		return nil, nil
	} else if response.Result.VirtualMachine == nil {
		return nil, errors.New("deploy job completed, but no VM provided")
	} else {
		return response.Result.VirtualMachine, nil
	}
}

func (api *API) vmAction(id string, command string) error {
	params := map[string]string{"id": id}
	return api.request(command, params, nil)
}

func (api *API) StartVirtualMachine(id string) error {
	return api.vmAction(id, "startVirtualMachine")
}

func (api *API) StopVirtualMachine(id string) error {
	return api.vmAction(id, "stopVirtualMachine")
}

func (api *API) RebootVirtualMachine(id string) error {
	return api.vmAction(id, "rebootVirtualMachine")
}

func (api *API) DestroyVirtualMachine(id string, expunge bool) error {
	params := map[string]string{"id": id}
	if expunge {
		params["expunge"] = "true"
	}
	return api.request("destroyVirtualMachine", params, nil)
}

func (api *API) GetVirtualMachine(id string) (*APIVirtualMachine, error) {
	params := map[string]string{"id": id}
	var response APIListVirtualMachinesResponse
	err := api.request("listVirtualMachines", params, &response)
	if err != nil {
		return nil, err
	} else if len(response.VirtualMachines) != 1 {
		return nil, fmt.Errorf("failed to get VM %s: response contains %d VMs, expected 1", id, len(response.VirtualMachines))
	} else {
		return &response.VirtualMachines[0], nil
	}
}

func (api *API) CreateVMSnapshot(id string) (string, error) {
	params := map[string]string{"virtualmachineid": id}
	var response APIIDResponse
	err := api.request("createVMSnapshot", params, &response)
	if err != nil {
		return "", err
	} else {
		return response.ID, nil
	}
}
