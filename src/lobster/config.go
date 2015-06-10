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
	StorageFee float64
	Currency string
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

	// do some basic checking
	if len(cfg.Default.Currency) != 3 {
		log.Printf("Warning: currency is set to [%s], but currency codes should be three characters", cfg.Default.Currency)
	}
	if cfg.Default.BandwidthOverageFee == 0 {
		log.Printf("Warning: bandwidth overage fee not set")
	}
	if cfg.Default.StorageFee == 0 {
		log.Printf("Warning: storage fee not set")
	}

	return &cfg
}
