package olt

// CardInfo holds data about physical cards in OLT
type CardInfo struct {
	Rack     string `json:"rack"`
	Shelf    string `json:"shelf"`
	Slot     string `json:"slot"`
	CfgType  string `json:"cfg_type"`
	RealType string `json:"real_type"`
	Port     string `json:"port"`
	HardVer  string `json:"hard_ver"`
	SoftVer  string `json:"soft_ver"`
	Status   string `json:"status"` // INSERVICE, NOPOWER, OFFLINE, STANDBY
}

// PONPort holds summary of a PON interface
type PONPort struct {
	Name       string `json:"name"` // e.g. "1/3/1"
	FullIntf   string `json:"full_intf"` // e.g. "gpon-olt_1/3/1"
	AdminState string `json:"admin_state"` // enable / disable
	LinkState  string `json:"link_state"`  // up / down
	TotalONUs  int    `json:"total_onus"`
	OnlineONUs int    `json:"online_onus"`
}

// ONUInfo holds status and provisioning details of an ONU
type ONUInfo struct {
	Interface    string `json:"interface"`     // e.g. "gpon-onu_1/3/3:3"
	Port         string `json:"port"`          // e.g. "1/3/3"
	OnuID        string `json:"onu_id"`        // e.g. "3"
	Name         string `json:"name"`          // Customer Name
	Description  string `json:"description"`   // Customer Description/PPPoE
	SerialNumber string `json:"serial_number"` // SN (e.g. ZTEG00000001 / HWTC00000002)
	ModelType    string `json:"model_type"`    // e.g. ALL, HG8245H5
	AdminState   string `json:"admin_state"`   // enable / disable
	OmciState    string `json:"omci_state"`    // normal / initial
	PhaseState   string `json:"phase_state"`   // working, LOS, off-line, dying-gasp
	RxPower      string `json:"rx_power"`      // e.g. "-19.45 dBm"
	TxPower      string `json:"tx_power"`      // e.g. "2.35 dBm"
	WanIP        string `json:"wan_ip"`        // PPPoE assigned IP
	Vlan         string `json:"vlan"`          // VLAN ID
}

// UncfgONU represents an unconfigured ONU detected on a PON port
type UncfgONU struct {
	Port         string `json:"port"`          // e.g. "1/3/3"
	OnuID        string `json:"onu_id"`        // Suggested ONU ID or detected index
	SerialNumber string `json:"serial_number"` // SN
	ModelType    string `json:"model_type"`    // detected vendor/type
}

// OpticalPower details
type OpticalPower struct {
	RxPower     string `json:"rx_power"`     // Rx at ONU (-15 to -24 dBm is good)
	OnuRx       string `json:"onu_rx"`       // Rx at ONU
	TxPower     string `json:"tx_power"`     // Tx at ONU
	OltRxPower  string `json:"olt_rx_power"` // Rx at OLT
	OltRx       string `json:"olt_rx"`       // Rx at OLT
	Attenuation string `json:"attenuation"`  // Total loss in dB
	Status      string `json:"status"`       // "GOOD", "WARNING", "CRITICAL", "OFFLINE"
}

// ProvisionRequest holds input parameters to register an ONU
type ProvisionRequest struct {
	Port         string `json:"port"`          // e.g. "1/3/3"
	OnuID        string `json:"onu_id"`        // e.g. "3"
	SerialNumber string `json:"serial_number"` // SN e.g. "ZTEG00000001"
	Name         string `json:"name"`          // Customer Name (Optional)
	VlanID       string `json:"vlan_id"`       // e.g. "100"
	VlanProfile  string `json:"vlan_profile"`  // e.g. "PPPOE-100"
	Username     string `json:"username"`      // PPPoE username
	Password     string `json:"password"`      // PPPoE password
	WiFiSSID     string `json:"wifi_ssid"`     // Optional Wi-Fi SSID
	WiFiPassword string `json:"wifi_password"` // Optional Wi-Fi Password
}

// TcontProfile represents a bandwidth upstream profile on ZTE OLT
type TcontProfile struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	FBW      string `json:"fbw"`
	ABW      string `json:"abw"`
	MBW      string `json:"mbw"`
	Priority string `json:"priority"`
	Weight   string `json:"weight"`
}

// TrafficProfile represents a downstream/gemport traffic shape profile on ZTE OLT
type TrafficProfile struct {
	Name string `json:"name"`
	SIR  string `json:"sir"`
	PIR  string `json:"pir"`
	CBS  string `json:"cbs"`
	PBS  string `json:"pbs"`
}

// VlanProfile represents a VLAN tag profile for ONUs
type VlanProfile struct {
	Name     string `json:"name"`
	TagMode  string `json:"tag_mode"`
	CVLAN    string `json:"cvlan"`
	Priority string `json:"priority"`
}

// VlanItem represents a configured VLAN on the OLT switch engine
type VlanItem struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	ONUs  int    `json:"onus"`
}

// OLTFullConfig aggregates all live profiles fetched from the hardware
type OLTFullConfig struct {
	Vlans           []VlanItem       `json:"vlans"`
	VlanProfiles    []VlanProfile    `json:"vlan_profiles"`
	TcontProfiles   []TcontProfile   `json:"tcont_profiles"`
	TrafficProfiles []TrafficProfile `json:"traffic_profiles"`
}

// ONTDetail aggregates all live hardware parameters, optical telemetry, and PPPoE/Wi-Fi configs of a specific ONT
type ONTDetail struct {
	Interface      string        `json:"interface"`       // e.g. "gpon-onu_1/3/1:1"
	Port           string        `json:"port"`            // e.g. "1/3/1"
	OnuID          string        `json:"onu_id"`          // e.g. "1"
	Name           string        `json:"name"`            // Customer Name
	Description    string        `json:"description"`     // Description / Area
	SerialNumber   string        `json:"serial_number"`   // SN
	ModelType      string        `json:"model_type"`      // Model
	AdminState     string        `json:"admin_state"`     // enable / disable
	PhaseState     string        `json:"phase_state"`     // working / dyinggasp / los
	Distance       string        `json:"distance"`        // e.g. "1004m"
	OnlineDuration string        `json:"online_duration"` // e.g. "15h 42m 14s"
	AuthPassTime   string        `json:"auth_pass_time"`
	OfflineTime    string        `json:"offline_time"`
	WanIP          string        `json:"wan_ip"`          // Current IP
	PPPoEUser      string        `json:"pppoe_user"`      // PPPoE Username
	PPPoEPass      string        `json:"pppoe_pass"`      // PPPoE Password
	VlanProfile     string        `json:"vlan_profile"`    // VLAN Profile name
	CVLAN           string        `json:"cvlan"`           // CVLAN ID
	UploadProfile   string        `json:"upload_profile"`   // TCONT Profile
	DownloadProfile string        `json:"download_profile"` // Traffic Gemport Profile
	MACAddress      string        `json:"mac_address"`     // MAC
	WiFiSSID        string        `json:"wifi_ssid"`       // Wi-Fi SSID
	WiFiPassword    string        `json:"wifi_password"`   // Wi-Fi Password
	OpticalPower    *OpticalPower `json:"optical_power"`   // Live Optical Power
}

// ONTUpdateRequest holds parameters to update ONT PPPoE, VLAN or Wi-Fi settings
type ONTUpdateRequest struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	PPPoEUser       string `json:"pppoe_user"`
	PPPoEPass       string `json:"pppoe_pass"`
	VlanProfile     string `json:"vlan_profile"`
	VlanID          string `json:"vlan_id"`
	UploadProfile   string `json:"upload_profile"`
	DownloadProfile string `json:"download_profile"`
	WiFiSSID        string `json:"wifi_ssid"`
	WiFiPassword    string `json:"wifi_password"`
}
