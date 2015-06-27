package solusvm

import "bytes"
import "crypto/rand"
import "crypto/tls"
import "encoding/xml"
import "errors"
import "fmt"
import "io/ioutil"
import "net"
import "net/http"
import "net/url"
import "strconv"
import "time"

type API struct {
	Url string
	ApiId string
	ApiKey string
	Insecure bool // InsecureSkipVerify true in tls.Config
}

func (this *API) uid() string {
	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	str := make([]rune, len(bytes))
	for i := range bytes {
		str[i] = alphabet[int(bytes[i]) % len(alphabet)]
	}
	return string(str)
}

func (this *API) request(action string, params map[string]string, target interface{}) error {
	// get raw parameters string
	if params == nil {
		params = make(map[string]string)
	}
	params["action"] = action
	params["id"] = this.ApiId
	params["key"] = this.ApiKey

	// perform request
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	byteBuffer := new(bytes.Buffer)
	byteBuffer.Write([]byte(values.Encode()))

	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: this.Insecure,
			},
			Dial: (&net.Dialer{
				Timeout: 30 * time.Second,
			}).Dial,
		},
	}
	response, err := c.PostForm(this.Url, values)

	if err != nil {
		return err
	}
	responseBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}
	response.Body.Close()
	responseRooted := "<root>" + string(responseBytes) + "</root>"

	// decode XML
	// we first decode into generic response for error checking; then into specific response to return
	var genericResponse APIGenericResponse
	err = xml.Unmarshal([]byte(responseRooted), &genericResponse)

	if err != nil {
		return err
	} else if genericResponse.Status != "success" {
		if genericResponse.Message != "" {
			return errors.New(genericResponse.Message)
		} else {
			return errors.New("backend call failed for unknown reason")
		}
	}

	if target != nil {
		err = xml.Unmarshal([]byte(responseRooted), target)
		if err != nil {
			return err
		}
	}

	return nil
}

// virtual machines

func (this *API) VmCreate(virtType string, nodeGroup string, hostname string, imageIdentification string, memory int, diskspace int, cpu int) (int, string, error) {
	rootPassword := this.uid()
	params := make(map[string]string)
	params["type"] = virtType
	params["nodegroup"] = nodeGroup
	params["hostname"] = hostname
	params["password"] = rootPassword
	params["username"] = "lobster"
	params["plan"] = "Lobster " + virtType
	params["template"] = imageIdentification
	params["ips"] = "1"

	// SolusVM complains about bad "Custom burst memory" if we set custommemory
	// however their documentation does not specify any way to provide a custom burst memory
	// currently we work around this by adjusting the memory after the VM is provisioned
	// TODO: open ticket with SolusVM and see if there's better way!
	if virtType != "openvz" {
		params["custommemory"] = fmt.Sprintf("%d", memory)
	}
	params["customdiskspace"] = fmt.Sprintf("%d", diskspace)
	params["custombandwidth"] = "99999"
	params["customcpu"] = fmt.Sprintf("%d", cpu)
	var response APIVmCreateResponse
	err := this.request("vserver-create", params, &response)
	if err != nil {
		return 0, "", err
	} else {
		vmId, err := strconv.ParseInt(response.VmId, 10, 32)
		if err != nil {
			return 0, "", err
		} else {
			// apply custom memory work-around described above
			// we sleep for a bit to give time for provisioning
			// TODO: reportError?
			go func() {
				time.Sleep(15 * time.Second)
				this.VmStop(int(vmId))
				time.Sleep(time.Second)
				params := make(map[string]string)
				params["memory"] = fmt.Sprintf("%d|%d", memory, memory)
				this.vmAction(int(vmId), "vserver-change-memory", params)
				time.Sleep(5 * time.Second)
				this.VmStart(int(vmId))
			}()

			return int(vmId), rootPassword, nil
		}
	}
}

func (this *API) vmAction(vmIdentification int, action string, params map[string]string) error {
	if params == nil {
		params = make(map[string]string)
	}
	params["vserverid"] = fmt.Sprintf("%d", vmIdentification)
	return this.request(action, params, nil)
}

func (this *API) VmStart(vmIdentification int) error {
	return this.vmAction(vmIdentification, "vserver-boot", nil)
}

func (this *API) VmStop(vmIdentification int) error {
	return this.vmAction(vmIdentification, "vserver-shutdown", nil)
}

func (this *API) VmReboot(vmIdentification int) error {
	return this.vmAction(vmIdentification, "vserver-reboot", nil)
}

func (this *API) VmDelete(vmIdentification int) error {
	params := make(map[string]string)
	params["deleteclient"] = "false"
	return this.vmAction(vmIdentification, "vserver-terminate", params)
}

func (this *API) VmReimage(vmIdentification int, imageIdentification string) error {
	params := make(map[string]string)
	params["template"] = imageIdentification
	return this.vmAction(vmIdentification, "vserver-rebuild", params)
}

func (this *API) VmHostname(vmIdentification int, hostname string) error {
	params := make(map[string]string)
	params["hostname"] = hostname
	return this.vmAction(vmIdentification, "vserver-hostname", params)
}

func (this *API) VmDiskSwap(vmIdentification int) error {
	return this.vmAction(vmIdentification, "diskswap", nil)
}

func (this *API) VmTunTap(vmIdentification int, enable bool) error {
	if enable {
		return this.vmAction(vmIdentification, "vserver-tun-enable", nil)
	} else {
		return this.vmAction(vmIdentification, "vserver-tun-disable", nil)
	}
}

func (this *API) VmVnc(vmIdentification int) (*APIVmVncResponse, error) {
	params := make(map[string]string)
	params["vserverid"] = fmt.Sprintf("%d", vmIdentification)
	var response APIVmVncResponse
	err := this.request("vserver-vnc", params, &response)
	if err != nil {
		return nil, err
	} else {
		return &response, nil
	}
}

func (this *API) VmConsole(vmIdentification int) (*APIVmConsoleResponse, error) {
	params := make(map[string]string)
	params["vserverid"] = fmt.Sprintf("%d", vmIdentification)
	var response APIVmConsoleResponse
	err := this.request("vserver-console", params, &response)
	if err != nil {
		return nil, err
	} else {
		return &response, nil
	}
}

func (this *API) VmInfo(vmIdentification int) (*APIVmInfoResponse, error) {
	params := make(map[string]string)
	params["vserverid"] = fmt.Sprintf("%d", vmIdentification)
	var response APIVmInfoResponse
	err := this.request("vserver-infoall", params, &response)
	return &response, err
}

func (this *API) VmAddAddress(vmIdentification int) error {
	return this.vmAction(vmIdentification, "vserver-addip", nil)
}

func (this *API) VmRemoveAddress(vmIdentification int, ip string) error {
	params := make(map[string]string)
	params["ipaddr"] = ip
	return this.vmAction(vmIdentification, "vserver-delip", params)
}

func (this *API) VmResizeDisk(vmIdentification int, hdd int) error {
	params := make(map[string]string)
	params["hdd"] = fmt.Sprintf("%d", hdd)
	return this.vmAction(vmIdentification, "vserver-change-hdd", params)
}

func (this *API) VmResizeMemory(vmIdentification int, memory int) error {
	params := make(map[string]string)
	params["memory"] = fmt.Sprintf("%d", memory)
	return this.vmAction(vmIdentification, "vserver-change-memory", params)
}

func (this *API) VmResizeCpu(vmIdentification int, cpu int) error {
	params := make(map[string]string)
	params["cpu"] = fmt.Sprintf("%d", cpu)
	return this.vmAction(vmIdentification, "vserver-change-cpu", params)
}
