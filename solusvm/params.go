package solusvm

import "encoding/xml"

type APIGenericResponse struct {
	XMLName xml.Name `xml:"root"`
	Status string `xml:"status"`
	Message string `xml:"statusmsg"`
}

// virtual machines

type APIVmCreateResponse struct {
	XMLName xml.Name `xml:"root"`
	VmId string `xml:"vserverid"`
}

type APIVmVncResponse struct {
	XMLName xml.Name `xml:"root"`
	Ip string `xml:"vncip"`
	Port string `xml:"vncport"`
	Password string `xml:"vncpassword"`
}

type APIVmInfoResponse struct {
	XMLName xml.Name `xml:"root"`
	Ip string `xml:"mainipaddress"`
	InternalIps string `xml:"internalips"`
	State string `xml:"state"`
	Bandwidth string `xml:"bandwidth"`
}
