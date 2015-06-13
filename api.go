package lobster

import "github.com/LunaNode/lobster/api"
import "github.com/LunaNode/lobster/ipaddr"
import "github.com/LunaNode/lobster/utils"

import "github.com/gorilla/mux"

import "crypto/hmac"
import "crypto/sha512"
import "crypto/subtle"
import "database/sql"
import "encoding/hex"
import "encoding/json"
import "errors"
import "fmt"
import "io"
import "net/http"
import "net/url"
import "strconv"
import "strings"
import "time"

type ApiKey struct {
	Id int
	Label string
	UserId int
	ApiId string
	CreatedTime time.Time
	Nonce int64

	// only set on apiCreate
	ApiKey string
}

func apiListHelper(rows *sql.Rows) []*ApiKey {
	var keys []*ApiKey
	defer rows.Close()
	for rows.Next() {
		var key ApiKey
		rows.Scan(&key.Id, &key.Label, &key.UserId, &key.ApiId, &key.CreatedTime, &key.Nonce)
		keys = append(keys, &key)
	}
	return keys
}

func apiList(db *Database, userId int) []*ApiKey {
	return apiListHelper(db.Query("SELECT id, label, user_id, api_id, time_created, nonce FROM api_keys WHERE user_id = ? ORDER BY label", userId))
}

func apiGet(db *Database, userId int, id int) *ApiKey {
	keys := apiListHelper(db.Query("SELECT id, label, user_id, api_id, time_created, nonce FROM api_keys WHERE user_id = ? AND id = ?", userId, id))
	if len(keys) == 1 {
		return keys[0]
	} else {
		return nil
	}
}

type ApiActionRestriction struct {
	Path string `json:"path"`
	Method string `json:"method"`
}

func apiCreate(db *Database, userId int, label string, restrictAction string, restrictIp string) (*ApiKey, error) {
	// validate restrictAction
	if len(restrictAction) > MAX_API_RESTRICTION {
		return nil, errors.New(fmt.Sprintf("action restriction JSON content cannot exceed %d characters", MAX_API_RESTRICTION))
	} else if restrictAction != "" {
		var actionRestrictions []*ApiActionRestriction
		err := json.Unmarshal([]byte(restrictAction), &actionRestrictions)
		if err != nil {
			return nil, err
		}
	}

	// validate restrictIp
	if len(restrictIp) > MAX_API_RESTRICTION {
		return nil, errors.New(fmt.Sprintf("IP restriction JSON content cannot exceed %d characters", MAX_API_RESTRICTION))
	} else if restrictIp != "" {
		_, err := ipaddr.ParseNetworks(restrictIp)
		if err != nil {
			return nil, err
		}
	}

	apiId := utils.Uid(16)
	apiKey := utils.Uid(128)
	result := db.Exec("INSERT INTO api_keys (label, user_id, api_id, api_key, restrict_action, restrict_ip) VALUES (?, ?, ?, ?, ?, ?)", label, userId, apiId, apiKey, restrictAction, restrictIp)
	id, _ := result.LastInsertId()
	key := apiGet(db, userId, int(id))
	key.ApiKey = apiKey
	return key, nil
}

func apiDelete(db *Database, userId int, id int) {
	db.Exec("DELETE FROM api_keys WHERE user_id = ? AND id = ?", userId, id)
}

type APIHandlerFunc func(http.ResponseWriter, *http.Request, *Database, int, []byte)

func apiCheck(db *Database, path string, method string, authorization string, request []byte, ip string) (int, error) {
	authParts := strings.Split(authorization, ":")
	if len(authParts) != 4 {
		return 0, errors.New(fmt.Sprintf("bad authorization: expected 4 semicolon-delimited parts, only found %d", len(authParts)))
	}

	apiId := authParts[0]
	apiPartialKey := authParts[1]
	nonce, _ := strconv.ParseInt(authParts[2], 10, 64)
	signature, _ := hex.DecodeString(authParts[3])

	if len(apiId) != 16 || len(apiPartialKey) != 64 || len(signature) != 64 {
		return 0, errors.New("bad authorization: id, partial key, or signature has bad length; or signature not hex-encoded")
	}

	rows := db.Query("SELECT api_keys.user_id, api_keys.api_key, api_keys.restrict_action, api_keys.restrict_ip FROM users, api_keys WHERE api_keys.api_id = ? AND api_keys.nonce < ? AND api_keys.user_id = users.id AND users.status != 'disabled'", apiId, nonce)
	defer rows.Close()
	if !rows.Next() {
		return 0, errors.New("authentication failure")
	}

	var userId int
	var actualKey, restrictAction, restrictIp string
	rows.Scan(&userId, &actualKey, &restrictAction, &restrictIp)

	// determine expected signature, hmac_{apikey}(path|nonce|request)
	mac := hmac.New(sha512.New, []byte(actualKey))
	toSign := fmt.Sprintf("%s|%d|%s", path, nonce, string(request))
	mac.Write([]byte(toSign))
	expectedSignature := mac.Sum(nil)

	partialGood := subtle.ConstantTimeCompare([]byte(actualKey)[:64], []byte(apiPartialKey)) == 1
	signatureGood := hmac.Equal(signature, expectedSignature)

	if partialGood && signatureGood {
		// now apply action and IP restrictions
		if restrictAction != "" {
			var actionRestrictions []*ApiActionRestriction
			err := json.Unmarshal([]byte(restrictAction), &actionRestrictions)
			if err != nil {
				return 0, err
			}

			passed := false
			for _, actionRestriction := range actionRestrictions {
				fmt.Printf("%s %s %s %s", actionRestriction.Method, method, actionRestriction.Path, path)
				if (actionRestriction.Method == "*" || actionRestriction.Method == method) && wildcardMatcher(actionRestriction.Path, path) {
					passed = true
					break
				}
			}

			if !passed {
				return 0, errors.New("failed action restriction")
			}
		}

		if restrictIp != "" && !ipaddr.MatchNetworks(restrictIp, ip) {
			return 0, errors.New("failed IP restriction")
		}

		db.Exec("UPDATE api_keys SET nonce = GREATEST(nonce, ?) WHERE api_id = ?", nonce, apiId)
		return userId, nil
	} else {
		return 0, errors.New("authentication failure")
	}
}

func apiWrap(h APIHandlerFunc) func(http.ResponseWriter, *http.Request, *Database) {
	return func(w http.ResponseWriter, r *http.Request, db *Database) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			http.Error(w, "Missing Authorization header", 401)
			return
		}

		authParts := strings.Split(authorization, " ")
		if (len(authParts) != 2 || authParts[0] != "lobster") && authParts[0] != "session" {
			http.Error(w, "Authorization header must take the form 'lobster authdata'", 400)
			return
		}

		apiPath := strings.Split(r.URL.Path, "/api/")[1]
		buf := make([]byte, API_MAX_REQUEST_LENGTH + 1)
		n, err := r.Body.Read(buf)
		if err != nil && err != io.EOF {
			http.Error(w, "Failed to read request body", 400)
			return
		} else if n > API_MAX_REQUEST_LENGTH {
			http.Error(w, fmt.Sprintf("Request body too long (max is %d)", API_MAX_REQUEST_LENGTH), 400)
			return
		}
		request := buf[:n]

		if authParts[0] == "lobster" {
			userId, err := apiCheck(db, apiPath, r.Method, authParts[1], request, extractIP(r.RemoteAddr))
			if err != nil {
				http.Error(w, err.Error(), 401)
				return
			}
			h(w, r, db, userId, request)
		} else if authParts[0] == "session" {
			// we modify request properties to ensure that session applies CSRF protection
			// TODO: this is a bit hacky
			r.Method = "POST"
			r.PostForm = url.Values{}
			r.PostForm.Set("token", authParts[1])
			sessionWrap(func(w http.ResponseWriter, r *http.Request, db *Database, session *Session) {
				if session.IsLoggedIn() {
					h(w, r, db, session.UserId, request)
				}
			})(w, r, db)
		}
	}
}

func apiResponse(w http.ResponseWriter, code int, v interface{}) {
	w.WriteHeader(code)
	if v != nil {
		bytes, err := json.Marshal(v)
		checkErr(err)
		w.Write(bytes)
	}
}

func copyVM(src *VirtualMachine, dst *api.VirtualMachine) {
	dst.Id = src.Id
	dst.PlanId = src.Plan.Id
	dst.Region = src.Region
	dst.Name = src.Name
	dst.Status = src.Status
	dst.TaskPending = src.TaskPending
	dst.ExternalIP = src.ExternalIP
	dst.PrivateIP = src.PrivateIP
	dst.CreatedTime = src.CreatedTime.Unix()
}

func copyVMDetails(src *VmInfo, dst *api.VirtualMachineDetails) {
	dst.Ip = src.Ip
	dst.PrivateIp = src.PrivateIp
	dst.Status = src.Status
	dst.Hostname = src.Hostname
	dst.BandwidthUsed = src.BandwidthUsed
	dst.LoginDetails = src.LoginDetails
	dst.Details = src.Details
	dst.CanVnc = src.CanVnc
	dst.CanReimage = src.CanReimage
	dst.CanSnapshot = src.CanSnapshot
	dst.CanAddresses = src.CanAddresses
	for _, srcAction := range src.Actions {
		dstAction := new(api.VirtualMachineAction)
		dstAction.Action = srcAction.Action
		dstAction.Name = srcAction.Name
		dstAction.Options = srcAction.Options
		dstAction.Description = srcAction.Description
		dstAction.Dangerous = srcAction.Dangerous
		dst.Actions = append(dst.Actions, dstAction)
	}
}

func copyAddress(src *IpAddress, dst *api.IpAddress) {
	dst.Ip = src.Ip
	dst.PrivateIp = src.PrivateIp
	dst.CanRdns = src.CanRdns
	dst.Hostname = src.Hostname
}

func copyImage(src *Image, dst *api.Image) {
	dst.Id = src.Id
	dst.Region = src.Region
	dst.Name = src.Name
	dst.Status = src.Status
}

func copyImageDetails(src *ImageInfo, dst *api.ImageDetails) {
	dst.Size = src.Size
	dst.Details = src.Details
	dst.Status = "unknown"
	if src.Status == ImagePending {
		dst.Status = "pending"
	} else if src.Status == ImageActive {
		dst.Status = "active"
	} else if src.Status == ImageError {
		dst.Status = "error"
	}
}

func copyPlan(src *Plan, dst *api.Plan) {
	dst.Id = src.Id
	dst.Name = src.Name
	dst.Price = src.Price
	dst.Ram = src.Ram
	dst.Cpu = src.Cpu
	dst.Storage = src.Storage
	dst.Bandwidth = src.Bandwidth
}

func apiVMList(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	var response api.VMListResponse
	for _, vm := range vmList(db, userId) {
		vmCopy := new(api.VirtualMachine)
		copyVM(vm, vmCopy)
		response.VirtualMachines = append(response.VirtualMachines, vmCopy)
	}
	apiResponse(w, 200, &response)
}

func apiVMCreate(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	var request api.VMCreateRequest

	err := json.Unmarshal(requestBytes, &request)
	if err != nil {
		http.Error(w, "Invalid json: " + err.Error(), 400)
		return
	}

	vmId, err := vmCreate(db, userId, request.Name, request.PlanId, request.ImageId)
	if err != nil {
		http.Error(w, "Create failed: " + err.Error(), 400)
		return
	} else {
		apiResponse(w, 201, api.VMCreateResponse{Id: vmId})
	}
}

func apiVMInfo(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}
	vm.LoadInfo()

	var response api.VMInfoResponse
	response.VirtualMachine = new(api.VirtualMachine)
	response.Details = new(api.VirtualMachineDetails)
	copyVM(vm, response.VirtualMachine)
	copyVMDetails(vm.Info, response.Details)
	apiResponse(w, 201, response)
}

func apiVMAction(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}

	var request api.VMActionRequest
	err = json.Unmarshal(requestBytes, &request)
	if err != nil {
		http.Error(w, "Invalid json: " + err.Error(), 400)
		return
	}

	err = nil
	var response interface{}
	if request.Action == "start" {
		err = vm.Start()
	} else if request.Action == "stop" {
		err = vm.Stop()
	} else if request.Action == "reboot" {
		err = vm.Reboot()
	} else if request.Action == "vnc" {
		var url string
		url, err = vm.Vnc()
		if err == nil {
			response = api.VMVncResponse{Url: url}
		}
	} else if request.Action == "rename" {
		err = vm.Rename(request.Value)
	} else if request.Action == "snapshot" {
		var imageId int
		imageId, err = vm.Snapshot(request.Value)
		if err == nil {
			response = api.VMSnapshotResponse{Id: imageId}
		}
	} else {
		err = vm.Action(request.Action, request.Value)
	}

	if err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		apiResponse(w, 200, response)
	}
}

func apiVMReimage(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}

	var request api.VMReimageRequest
	err = json.Unmarshal(requestBytes, &request)
	if err != nil {
		http.Error(w, "Invalid json: " + err.Error(), 400)
		return
	}

	err = vmReimage(db, userId, vm.Id, request.ImageId)
	if err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		apiResponse(w, 200, nil)
	}
}

func apiVMDelete(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}

	err = vm.Delete(userId)
	if err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		apiResponse(w, 204, nil)
	}
}

func apiVMAddresses(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}
	err = vm.LoadAddresses()
	if err != nil {
		http.Error(w, err.Error(), 400)
	}

	var response api.VMAddressesResponse
	for _, address := range vm.Addresses {
		addressCopy := new(api.IpAddress)
		copyAddress(address, addressCopy)
		response.Addresses = append(response.Addresses, addressCopy)
	}
	apiResponse(w, 200, &response)
}

func apiVMAddressAdd(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}

	err = vm.AddAddress()
	if err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		apiResponse(w, 200, nil)
	}
}

func apiVMAddressRemove(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}

	var request api.VMAddressRemoveRequest
	err = json.Unmarshal(requestBytes, &request)
	if err != nil {
		http.Error(w, "Invalid json: " + err.Error(), 400)
		return
	}

	err = vm.RemoveAddress(request.Ip, request.PrivateIp)
	if err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		apiResponse(w, 200, nil)
	}
}

func apiVMAddressRdns(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	vmId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid VM ID", 400)
		return
	}
	vm := vmGetUser(db, userId, int(vmId))
	if vm == nil {
		http.Error(w, "No virtual machine with that ID", 404)
		return
	}

	var request api.VMAddressRdnsRequest
	err = json.Unmarshal(requestBytes, &request)
	if err != nil {
		http.Error(w, "Invalid json: " + err.Error(), 400)
		return
	}

	err = vm.SetRdns(mux.Vars(r)["ip"], request.Hostname)
	if err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		apiResponse(w, 200, nil)
	}
}

func apiImageList(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	var response api.ImageListResponse
	for _, image := range imageList(db, userId) {
		imageCopy := new(api.Image)
		copyImage(image, imageCopy)
		response.Images = append(response.Images, imageCopy)
	}
	apiResponse(w, 200, &response)
}

func apiImageFetch(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	var request api.ImageFetchRequest

	err := json.Unmarshal(requestBytes, &request)
	if err != nil {
		http.Error(w, "Invalid json: " + err.Error(), 400)
		return
	}

	imageId, err := imageFetch(db, userId, request.Region, request.Name, request.Url, request.Format)
	if err != nil {
		http.Error(w, "Fetch failed: " + err.Error(), 400)
		return
	} else {
		apiResponse(w, 201, api.ImageFetchResponse{Id: imageId})
	}
}

func apiImageInfo(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	imageId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid image ID", 400)
		return
	}
	image := imageInfo(db, userId, int(imageId))
	if image == nil {
		http.Error(w, "No image with that ID", 404)
		return
	}

	var response api.ImageInfoResponse
	response.Image = new(api.Image)
	response.Details = new(api.ImageDetails)
	copyImage(image, response.Image)
	copyImageDetails(image.Info, response.Details)
	apiResponse(w, 201, response)
}

func apiImageDelete(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	imageId, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid image ID", 400)
		return
	}

	err = imageDelete(db, userId, int(imageId))
	if err != nil {
		http.Error(w, err.Error(), 400)
	} else {
		apiResponse(w, 204, nil)
	}
}

func apiPlanList(w http.ResponseWriter, r *http.Request, db *Database, userId int, requestBytes []byte) {
	var response api.PlanListResponse
	for _, plan := range planList(db) {
		planCopy := new(api.Plan)
		copyPlan(plan, planCopy)
		response.Plans = append(response.Plans, planCopy)
	}
	apiResponse(w, 200, &response)
}
