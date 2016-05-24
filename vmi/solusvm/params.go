package solusvm

import "encoding/xml"

type APIGenericResponse struct {
	XMLName xml.Name `xml:"root"`
	Status  string   `xml:"status"`
	Message string   `xml:"statusmsg"`
}

// virtual machines

type APIVmCreateResponse struct {
	XMLName      xml.Name `xml:"root"`
	VmId         string   `xml:"vserverid"`
	RootPassword string   `xml:"rootpassword"`
}

type APIVmVncResponse struct {
	XMLName  xml.Name `xml:"root"`
	Ip       string   `xml:"vncip"`
	Port     string   `xml:"vncport"`
	Password string   `xml:"vncpassword"`
}

type APIVmConsoleResponse struct {
	XMLName  xml.Name `xml:"root"`
	Ip       string   `xml:"consoleip"`
	Port     string   `xml:"consoleport"`
	Username string   `xml:"consoleusername"`
	Password string   `xml:"consolepassword"`
}

type APIVmInfoResponse struct {
	XMLName     xml.Name `xml:"root"`
	Ip          string   `xml:"mainipaddress"`
	Ips         string   `xml:"ipaddresses"`
	InternalIps string   `xml:"internalips"`
	State       string   `xml:"state"`
	Bandwidth   string   `xml:"bandwidth"`
}
