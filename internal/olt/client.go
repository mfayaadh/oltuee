package olt

// Client defines the common operations for managing an OLT
type Client interface {
	Connect() error
	Disconnect() error
	Close() error
	IsConnected() bool
	GetHostname() string

	// Hardware & System
	GetCards() ([]CardInfo, error)
	GetPONPorts() ([]PONPort, error)

	// ONU Management
	GetONUs(port string) ([]ONUInfo, error)
	GetAllONUs() ([]ONUInfo, error)
	GetUnconfiguredONUs() ([]UncfgONU, error)
	GetOpticalPower(onuIntf string) (*OpticalPower, error)
	GetWanIP(onuIntf string) (string, error)

	// Actions & Diagnostics
	ProvisionONU(req ProvisionRequest) error
	DeleteONU(port string, onuID string) error
	RebootONU(onuIntf string) error
	ConfigureWiFi(onuIntf, ssid, password string) error
	GetFullConfig() (*OLTFullConfig, error)
	GetONTDetail(onuIntf string) (*ONTDetail, error)
	UpdateONTConfig(onuIntf string, req ONTUpdateRequest) error
	RestoreFactoryONT(onuIntf string) error
	SetAdminState(onuIntf string, state string) error
	ExecRawCLI(cmd string) (string, error)
}
