package lobster

import "code.google.com/p/gcfg"
import "log"

type ConfigDefault struct {
	UrlBase string
	AdminEmail string
	FromEmail string
	ProxyHeader string
	Debug bool
	BandwidthOverageFee float64
}

type ConfigSession struct {
	Domain string
	Secure bool
}

type ConfigDatabase struct {
	Host string
	Username string
	Password string
	Name string
}

type ConfigHttp struct {
	Addr string
}

type ConfigNovnc struct {
	Url string
	Listen string
}

type Config struct {
	Default ConfigDefault
	Session ConfigSession
	Database ConfigDatabase
	Http ConfigHttp
	Novnc ConfigNovnc
}

func LoadConfig(cfgPath string) *Config {
	var cfg Config
	err := gcfg.ReadFileInto(&cfg, cfgPath)
	if err != nil {
		log.Printf("Error while reading configuration: %s", err.Error())
		panic(err)
	}
	return &cfg
}
