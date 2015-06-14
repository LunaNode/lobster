package lobster

import "code.google.com/p/gcfg"

import "log"

type ConfigDefault struct {
	UrlBase string
	AdminEmail string
	FromEmail string
	ProxyHeader string
	Debug bool
	Language string
}

type ConfigVm struct {
	MaximumIps int
}

type ConfigBilling struct {
	BandwidthOverageFee float64
	StorageFee float64
	Currency string
	BillingInterval int
	BillingVmMinimum int
	DepositMinimum float64
	DepositMaximum float64
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

type ConfigEmail struct {
	Host string
	Port int
	NoTLS bool
}

type ConfigNovnc struct {
	Url string
	Listen string
}

type Config struct {
	Default ConfigDefault
	Vm ConfigVm
	Billing ConfigBilling
	Session ConfigSession
	Database ConfigDatabase
	Http ConfigHttp
	Email ConfigEmail
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
	if len(cfg.Billing.Currency) != 3 {
		log.Printf("Warning: currency is set to [%s], but currency codes should be three characters", cfg.Billing.Currency)
	}
	if cfg.Billing.BandwidthOverageFee == 0 {
		log.Printf("Warning: bandwidth overage fee not set")
	}
	if cfg.Billing.StorageFee == 0 {
		log.Printf("Warning: storage fee not set")
	}
	if cfg.Billing.BillingInterval == 0 {
		log.Printf("Warning: billing interval not set, defaulting to 60 minutes")
		cfg.Billing.BillingInterval = 60
	}
	if cfg.Billing.BillingVmMinimum < 1 {
		log.Printf("Warning: minimum VM billing intervals less than 1, setting to 1")
		cfg.Billing.BillingVmMinimum = 1
	}

	return &cfg
}
