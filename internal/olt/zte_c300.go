package olt

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mfayaadh/oltuee/internal/config"
)

// ONUProfileCache holds parsed name, pppoe user, and description per ONU interface
type ONUProfileCache struct {
	Name        string `json:"name"`
	PPPoE       string `json:"pppoe"`
	Description string `json:"description"`
	SN          string `json:"sn,omitempty"`
	Model       string `json:"model,omitempty"`
	State       string `json:"state,omitempty"`
	RxPower     string `json:"rx_power,omitempty"`
}

// ZTEC300Driver implements Client interface for ZTE C300 / C320 / C600
type ZTEC300Driver struct {
	host         string
	port         string
	username     string
	password     string
	telnet       *TelnetClient
	hostname     string
	connected    bool
	profileCache map[string]ONUProfileCache // key: "1/3/1:3" -> cache
	cacheMu      sync.RWMutex
}

func getCacheDir() string {
	dataDir, err := config.DataDir()
	if err != nil {
		dataDir = "./data"
	}
	cacheDir := filepath.Join(dataDir, "cache")
	_ = os.MkdirAll(cacheDir, 0700)
	return cacheDir
}

func getCacheFilePath(host string) string {
	safeHost := strings.ReplaceAll(host, ":", "_")
	return filepath.Join(getCacheDir(), safeHost+".json")
}

func loadDiskCache(host string) map[string]ONUProfileCache {
	filePath := getCacheFilePath(host)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return make(map[string]ONUProfileCache)
	}
	var cache map[string]ONUProfileCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return make(map[string]ONUProfileCache)
	}
	return cache
}

func saveDiskCache(host string, cache map[string]ONUProfileCache) {
	filePath := getCacheFilePath(host)
	data, err := json.MarshalIndent(cache, "", "  ")
	if err == nil {
		_ = os.WriteFile(filePath, data, 0600)
	}
}

func NewZTEC300Driver(host, port, username, password string) *ZTEC300Driver {
	diskCache := loadDiskCache(host)
	return &ZTEC300Driver{
		host:         host,
		port:         port,
		username:     username,
		password:     password,
		profileCache: diskCache,
	}
}

func (z *ZTEC300Driver) Connect() error {
	tn, err := NewTelnetClient(z.host, z.port, 10*time.Second)
	if err != nil {
		return err
	}
	z.telnet = tn

	// Wait for Username/Login prompt
	out, err := tn.ReadUntilLogin("Username:", "username:", "login:", "#", ">")
	if err != nil {
		tn.Close()
		return fmt.Errorf("gagal menerima prompt awal OLT: %w", err)
	}

	if strings.Contains(strings.ToLower(out), "sername") || strings.Contains(strings.ToLower(out), "login") {
		if err := tn.sendLine(z.username); err != nil {
			tn.Close()
			return err
		}
		_, err = tn.ReadUntilLogin("Password:", "password:")
		if err != nil {
			tn.Close()
			return fmt.Errorf("gagal menerima prompt password: %w", err)
		}
		if err := tn.sendLine(z.password); err != nil {
			tn.Close()
			return err
		}
	}

	// Wait for CLI prompt
	cliOut := tn.readRaw(3*time.Second, 10*time.Second, func(s string) bool {
		return tn.looksLikePromptAtEnd(s)
	})

	if strings.Contains(strings.ToLower(cliOut), "failed") || strings.Contains(strings.ToLower(cliOut), "invalid") || strings.Contains(strings.ToLower(cliOut), "denied") {
		tn.Close()
		return errors.New("autentikasi ditolak: username atau password salah")
	}

	// Extract hostname from the prompt at the end of the output
	promptRe := regexp.MustCompile(`([A-Za-z0-9\-_]+)[#>]\s*$`)
	if m := promptRe.FindStringSubmatch(strings.TrimSpace(cliOut)); len(m) > 1 {
		z.hostname = m[1]
	} else {
		z.hostname = "ZTE-OLT"
	}
	tn.SetHostname(z.hostname)

	// Disable pagination
	_, _ = tn.ExecCommand("terminal length 0")

	z.connected = true

	return nil
}

// syncAllProfilesInBackground runs on a dedicated background telnet connection
// so the user's interactive session is NEVER blocked.
func (z *ZTEC300Driver) syncAllProfilesInBackground() {
	tn, err := NewTelnetClient(z.host, z.port, 10*time.Second)
	if err != nil {
		return
	}
	defer tn.Close()

	out, err := tn.ReadUntilLogin("Username:", "username:", "login:", "#", ">")
	if err != nil {
		return
	}
	if strings.Contains(strings.ToLower(out), "sername") || strings.Contains(strings.ToLower(out), "login") {
		if err := tn.sendLine(z.username); err != nil {
			return
		}
		_, _ = tn.ReadUntilLogin("Password:", "password:")
		if err := tn.sendLine(z.password); err != nil {
			return
		}
	}
	_ = tn.readRaw(2*time.Second, 10*time.Second, func(s string) bool { return tn.looksLikePromptAtEnd(s) })
	tn.SetHostname(z.hostname)
	_, _ = tn.ExecCommand("terminal length 0")

	// Execute full running-config sync on dedicated background connection
	cfgOut, err := tn.ExecCommand("show running-config | include interface gpon-onu_|  name |  description ", 60*time.Second)
	if err != nil || cfgOut == "" {
		return
	}

	onuRe := regexp.MustCompile(`interface gpon-onu_(\d+/\d+/\d+:\d+)`)
	dataMap := make(map[string]*ONUProfileCache)
	var curKey string

	for _, line := range strings.Split(cfgOut, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "interface gpon-onu_") {
			if m := onuRe.FindStringSubmatch(line); len(m) >= 2 {
				curKey = m[1]
				if _, ok := dataMap[curKey]; !ok {
					dataMap[curKey] = &ONUProfileCache{}
				}
			}
			continue
		}
		if curKey == "" {
			continue
		}
		if strings.HasPrefix(line, "name ") {
			if d, ok := dataMap[curKey]; ok {
				nm, ppp := parseCustomerName(strings.TrimPrefix(line, "name "))
				d.Name = nm
				if ppp != "" {
					d.PPPoE = ppp
				}
			}
		}
		if strings.HasPrefix(line, "description ") && !strings.HasPrefix(line, "service-port") {
			if d, ok := dataMap[curKey]; ok {
				d.Description = strings.TrimPrefix(line, "description ")
				if d.Name == "" {
					nm, ppp := parseCustomerDescription(d.Description)
					d.Name = nm
					if d.PPPoE == "" && ppp != "" {
						d.PPPoE = ppp
					}
				}
			}
		}
	}

	// Also fetch and parse all real Serial Numbers from 'show gpon onu baseinfo gpon-olt_X/Y/Z'
	activePorts := make(map[string]bool)
	for k := range dataMap {
		parts := strings.Split(k, ":")
		if len(parts) >= 1 {
			activePorts[parts[0]] = true
		}
	}

	baseRe := regexp.MustCompile(`gpon-onu_(\d+/\d+/\d+:\d+)\s+(\S+)\s+\S+\s+SN:([A-Za-z0-9]+)`)
	for port := range activePorts {
		cmd := "show gpon onu baseinfo gpon-olt_" + port
		if baseOut, err := tn.ExecCommand(cmd, 6*time.Second); err == nil && baseOut != "" {
			for _, line := range strings.Split(baseOut, "\n") {
				line = strings.TrimSpace(line)
				if m := baseRe.FindStringSubmatch(line); len(m) >= 4 {
					onuKey := m[1]
					model := m[2]
					sn := m[3]
					if d, ok := dataMap[onuKey]; ok {
						d.SN = sn
						d.Model = model
					} else {
						dataMap[onuKey] = &ONUProfileCache{
							SN:    sn,
							Model: model,
						}
					}
				}
			}
		}
	}

	z.cacheMu.Lock()
	for k, v := range dataMap {
		cur := z.profileCache[k]
		if v.Name != "" {
			cur.Name = v.Name
		}
		if v.PPPoE != "" {
			cur.PPPoE = v.PPPoE
		}
		if v.Description != "" {
			cur.Description = v.Description
		}
		if v.SN != "" {
			cur.SN = v.SN
		}
		if v.Model != "" {
			cur.Model = v.Model
		}
		z.profileCache[k] = cur
	}
	saveDiskCache(z.host, z.profileCache)
	z.cacheMu.Unlock()
}

func (z *ZTEC300Driver) Disconnect() error {
	z.connected = false
	if z.telnet != nil {
		// Send exit without waiting for response - socket will close
		_ = z.telnet.sendLine("exit")
		err := z.telnet.Close()
		z.telnet = nil
		return err
	}
	return nil
}

func (z *ZTEC300Driver) Close() error {
	return z.Disconnect()
}

func (z *ZTEC300Driver) IsConnected() bool { return z.connected }
func (z *ZTEC300Driver) GetHostname() string { return z.hostname }

// GetCards parses 'show card'
// Actual output format:
// Rack Shelf Slot CfgType RealType Port  HardVer SoftVer         Status
// -------------------------------------------------------------------------------
// 1    1     0    PRWH    PRWH     0     V1.0.0                  INSERVICE
// 1    1     3    GTGH    GTGHG    16    V1.0.0  V2.1.0          INSERVICE
func (z *ZTEC300Driver) GetCards() ([]CardInfo, error) {
	if !z.connected {
		return nil, errors.New("tidak terhubung ke OLT")
	}

	out, err := z.telnet.ExecCommand("show card")
	if err != nil {
		return nil, err
	}

	var cards []CardInfo
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Rack") || strings.HasPrefix(l, "---") {
			continue
		}
		fields := strings.Fields(l)
		if len(fields) >= 5 {
			card := CardInfo{
				Rack:     fields[0],
				Shelf:    fields[1],
				Slot:     fields[2],
				CfgType:  fields[3],
				RealType: fields[4],
				Status:   fields[len(fields)-1],
			}
			if len(fields) >= 7 {
				card.Port = fields[5]
			}
			cards = append(cards, card)
		}
	}
	return cards, nil
}

// GetPONPorts parses 'show gpon olt interface'
func (z *ZTEC300Driver) GetPONPorts() ([]PONPort, error) {
	if !z.connected {
		return nil, errors.New("tidak terhubung ke OLT")
	}

	out, err := z.telnet.ExecCommand("show gpon olt interface", 10*time.Second)
	if err != nil {
		return nil, err
	}

	var ports []PONPort
	lines := strings.Split(out, "\n")
	re := regexp.MustCompile(`gpon-olt_(\d+/\d+/\d+)\s+(\S+)\s+(\S+)`)
	for _, l := range lines {
		if m := re.FindStringSubmatch(l); len(m) == 4 {
			ports = append(ports, PONPort{
				Name:       m[1],
				FullIntf:   "gpon-olt_" + m[1],
				AdminState: m[2],
				LinkState:  m[3],
			})
		}
	}
	return ports, nil
}

// GetONUs retrieves ONUs on a specific port.
// Actual ZTE output format for `show gpon onu state gpon-olt_1/3/1`:
//
//	OnuIndex   Admin State  OMCC State  Phase State  Channel
//	--------------------------------------------------------------
//	1/3/1:1     enable       enable      working      1(GPON)
//	1/3/1:2     enable       disable     DyingGasp    1(GPON)
//	1/3/1:42    enable       disable     LOS          1(GPON)
//	ONU Number: 46/49
func (z *ZTEC300Driver) GetONUs(port string) ([]ONUInfo, error) {
	if !z.connected {
		return nil, errors.New("tidak terhubung ke OLT")
	}

	intfName := port
	if !strings.HasPrefix(port, "gpon-olt_") {
		intfName = "gpon-olt_" + port
	}

	out, err := z.telnet.ExecCommand("show gpon onu state "+intfName, 10*time.Second)
	if err != nil {
		return nil, err
	}

	var onus []ONUInfo
	lines := strings.Split(out, "\n")
	// Match: "1/3/1:1     enable       enable      working      1(GPON)"
	re := regexp.MustCompile(`(\d+/\d+/\d+):(\d+)\s+(\S+)\s+(\S+)\s+(\S+)\s+`)
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Onu") || strings.HasPrefix(l, "---") || strings.HasPrefix(l, "ONU Number") {
			continue
		}
		if m := re.FindStringSubmatch(l); len(m) >= 6 {
			onu := ONUInfo{
				Interface:  fmt.Sprintf("gpon-onu_%s:%s", m[1], m[2]),
				Port:       m[1],
				OnuID:      m[2],
				AdminState: m[3],
				OmciState:  m[4],
				PhaseState: m[5],
			}
			onus = append(onus, onu)
		}
	}

	// 1. Fetch Serial Numbers from baseinfo (super fast 140ms!)
	z.enrichWithBaseInfo(onus, intfName)

	// 2. Instant cache merge (0ms)
	z.enrichWithDescription(onus, port)

	return onus, nil
}

// enrichWithBaseInfo fetches SN data from `show gpon onu baseinfo`
// Actual format:
//
//	gpon-onu_1/3/1:1    ALL         sn      SN:ZTEG00000001         ready
//	gpon-onu_1/3/1:18   HG8245H5    sn      SN:HWTC00000002         ready
func (z *ZTEC300Driver) enrichWithBaseInfo(onus []ONUInfo, intfName string) {
	if len(onus) == 0 {
		return
	}

	baseOut, err := z.telnet.ExecCommand("show gpon onu baseinfo "+intfName, 10*time.Second)
	if err != nil || baseOut == "" {
		return
	}

	// Build map: "1/3/1:1" -> {sn, type}
	type baseData struct {
		sn       string
		onuModel string
	}
	dataMap := make(map[string]baseData)

	// Match: "gpon-onu_1/3/1:1    ALL         sn      SN:ZTEG00000001         ready"
	snRe := regexp.MustCompile(`gpon-onu_(\d+/\d+/\d+:\d+)\s+(\S+)\s+\S+\s+SN:(\S+)\s+`)
	for _, line := range strings.Split(baseOut, "\n") {
		if m := snRe.FindStringSubmatch(line); len(m) >= 4 {
			dataMap[m[1]] = baseData{sn: m[3], onuModel: m[2]}
		}
	}

	for i := range onus {
		key := onus[i].Port + ":" + onus[i].OnuID
		if d, ok := dataMap[key]; ok {
			onus[i].SerialNumber = d.sn
onus[i].ModelType = d.onuModel
		}
	}
}

// enrichWithDescription merges cached customer names and PPPoE usernames.
// If names for this port are not in cache yet, it queries the port's interface config and saves to cache.
func (z *ZTEC300Driver) enrichWithDescription(onus []ONUInfo, port string) {
	if len(onus) == 0 {
		return
	}

	portBase := port
	if strings.HasPrefix(port, "gpon-olt_") {
		portBase = strings.TrimPrefix(port, "gpon-olt_")
	}

	// 1. Check if all ONUs for this port exist in cache
	missingCount := 0
	z.cacheMu.RLock()
	for i := range onus {
		key := onus[i].Port + ":" + onus[i].OnuID
		if c, ok := z.profileCache[key]; ok {
			if c.Name != "" {
				onus[i].Name = c.Name
			}
			if c.PPPoE != "" {
				onus[i].WanIP = c.PPPoE
			}
			if c.Description != "" {
				onus[i].Description = c.Description
			}
		} else {
			missingCount++
		}
	}
	z.cacheMu.RUnlock()

	// 2. If some ONUs are missing from cache (e.g. newly provisioned or uncached port):
	// Fetch interface names for this port and update cache
	if missingCount > 0 {
		nameOut, err := z.telnet.ExecCommand(
			fmt.Sprintf("show running-config | include gpon-onu_%s:|  name |  description ", portBase),
			10*time.Second,
		)
		if err == nil && nameOut != "" {
			z.parseNameAndDescription(onus, nameOut, portBase)
		}
	}
}

// parseNameAndDescription parses 'name' and 'description' fields from filtered running-config output.
func (z *ZTEC300Driver) parseNameAndDescription(onus []ONUInfo, output, portBase string) {
	type onuExtra struct {
		name string
		desc string
	}
	dataMap := make(map[string]*onuExtra)
	var currentKey string

	onuRe := regexp.MustCompile(`gpon-onu_\d+/\d+/\d+:(\d+)`)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Track which ONU we're in
		if strings.Contains(line, "gpon-onu_"+portBase+":") {
			if m := onuRe.FindStringSubmatch(line); len(m) >= 2 {
				currentKey = m[1]
				if _, ok := dataMap[currentKey]; !ok {
					dataMap[currentKey] = &onuExtra{}
				}
			}
			continue
		}

		if currentKey == "" {
			continue
		}

		// Parse "name 100000000001-Alice"
		if strings.HasPrefix(line, "name ") {
			if d, ok := dataMap[currentKey]; ok {
				d.name = strings.TrimPrefix(line, "name ")
			}
		}

		// Parse "description 3$$100000000001-Alice$$"
		// Skip "service-port N description XXX" lines
		if strings.HasPrefix(line, "description ") && !strings.HasPrefix(line, "service-port") {
			if d, ok := dataMap[currentKey]; ok {
				d.desc = strings.TrimPrefix(line, "description ")
			}
		}
	}

	// Update cache
	z.cacheMu.Lock()
	for id, d := range dataMap {
		fullKey := portBase + ":" + id
		var parsedName, parsedPPPoE string
		if d.name != "" {
			parsedName, parsedPPPoE = parseCustomerName(d.name)
		} else if d.desc != "" {
			parsedName, parsedPPPoE = parseCustomerDescription(d.desc)
		}
		cur := z.profileCache[fullKey]
		cur.Name = parsedName
		cur.PPPoE = parsedPPPoE
		cur.Description = d.desc
		for _, o := range onus {
			if o.OnuID == id {
				if o.SerialNumber != "" {
					cur.SN = o.SerialNumber
				}
				if o.ModelType != "" {
					cur.Model = o.ModelType
				}
				if o.PhaseState != "" {
					cur.State = o.PhaseState
				}
				break
			}
		}
		z.profileCache[fullKey] = cur
	}
	saveDiskCache(z.host, z.profileCache)
	z.cacheMu.Unlock()

	// Apply immediately to ONUs slice
	z.cacheMu.RLock()
	defer z.cacheMu.RUnlock()
	for i := range onus {
		fullKey := portBase + ":" + onus[i].OnuID
		if c, ok := z.profileCache[fullKey]; ok {
			if c.Name != "" {
				onus[i].Name = c.Name
			}
			if c.PPPoE != "" {
				onus[i].WanIP = c.PPPoE
			}
			if c.Description != "" {
				onus[i].Description = c.Description
			}
			c.SN = onus[i].SerialNumber
			c.State = onus[i].PhaseState
			c.RxPower = onus[i].RxPower
			z.profileCache[fullKey] = c
		}
	}
}

// parseCustomerName intelligently splits raw name strings into Customer Name and PPPoE Username.
// Handles formats like:
//   - "100000000001-Alice" -> Name: "Alice", PPPoE: "100000000001"
//   - "Bob-100000000002" -> Name: "Bob", PPPoE: "100000000002"
//   - "Charlie | 100000000003" -> Name: "Charlie", PPPoE: "100000000003"
//   - "100000000004" -> Name: "-", PPPoE: "100000000004"
//   - "Diana.A-100000000005" -> Name: "Diana.A", PPPoE: "100000000005"
//   - "78100000000006-Evan" -> Name: "Evan", PPPoE: "78100000000006"
func parseCustomerName(raw string) (name, pppoe string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}

	// Case 1: Pipe delimiter '|'
	if parts := strings.Split(raw, "|"); len(parts) >= 2 {
		p0 := strings.TrimSpace(parts[0])
		p1 := strings.TrimSpace(parts[1])
		if isMostlyDigits(p1) {
			return p0, p1
		} else if isMostlyDigits(p0) {
			return p1, p0
		}
		return p0, p1
	}

	// Case 2: Dash delimiter '-'
	if dashIdx := strings.Index(raw, "-"); dashIdx > 0 {
		p0 := strings.TrimSpace(raw[:dashIdx])
		p1 := strings.TrimSpace(raw[dashIdx+1:])
		if isMostlyDigits(p0) {
			return p1, p0
		} else if isMostlyDigits(p1) {
			return p0, p1
		}
		return p1, p0
	}

	// Case 3: All digits (pure PPPoE user)
	if isMostlyDigits(raw) {
		return "-", raw
	}

	// Case 4: Pure customer name
	return raw, ""
}

// parseCustomerDescription extracts customer name and PPPoE from description field (e.g. "3$$100000000001-Alice$$")
func parseCustomerDescription(raw string) (name, pppoe string) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "$$") {
		parts := strings.Split(raw, "$$")
		if len(parts) >= 2 && parts[1] != "" {
			return parseCustomerName(parts[1])
		}
	}
	return parseCustomerName(raw)
}

// isMostlyDigits checks if a string is composed predominantly of numeric digits (like a PPPoE username)
func isMostlyDigits(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	digitCount := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digitCount++
		}
	}
	return digitCount >= 8 || (len(s) > 0 && digitCount == len(s))
}

// parsePPPoEUsername parses PPPoE username from pon-onu-mng sections.
func (z *ZTEC300Driver) parsePPPoEUsername(onus []ONUInfo, output, portBase string) {
	pppoeMap := make(map[string]string) // onuID -> PPPoE username
	var currentKey string

	onuRe := regexp.MustCompile(`gpon-onu_\d+/\d+/\d+:(\d+)`)
	pppoeRe := regexp.MustCompile(`wan-ip\s+\d+\s+mode\s+pppoe\s+username\s+(\S+)`)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Track which ONU section
		if strings.Contains(line, "pon-onu-mng") && strings.Contains(line, portBase+":") {
			if m := onuRe.FindStringSubmatch(line); len(m) >= 2 {
				currentKey = m[1]
			}
			continue
		}

		// Parse "wan-ip 1 mode pppoe username 100000000001 password ..."
		if currentKey != "" && strings.Contains(line, "pppoe") {
			if m := pppoeRe.FindStringSubmatch(line); len(m) >= 2 {
				pppoeMap[currentKey] = m[1]
			}
		}
	}

	// Apply PPPoE usernames
	for i := range onus {
		if pppoe, ok := pppoeMap[onus[i].OnuID]; ok {
			onus[i].WanIP = pppoe
		}
	}
}

// GetAllONUs is not used in the new per-port flow but kept for compatibility
func (z *ZTEC300Driver) GetAllONUs() ([]ONUInfo, error) {
	ports, err := z.GetPONPorts()
	if err != nil {
		return nil, err
	}
	var allONUs []ONUInfo
	for _, p := range ports {
		if p.AdminState == "enable" {
			onus, err := z.GetONUs(p.Name)
			if err == nil {
				allONUs = append(allONUs, onus...)
			}
		}
	}
	return allONUs, nil
}

// GetUnconfiguredONUs parses 'show gpon onu uncfg'
// Actual format:
//
//	OnuIndex                 Sn                  State
//	---------------------------------------------------------------------
//	gpon-onu_1/3/1:1         HWTC00000001        unknown
func (z *ZTEC300Driver) GetUnconfiguredONUs() ([]UncfgONU, error) {
	if !z.connected {
		return nil, errors.New("tidak terhubung ke OLT")
	}

	out, err := z.telnet.ExecCommand("show gpon onu uncfg", 15*time.Second)
	if err != nil {
		return nil, err
	}

	var uncfgs []UncfgONU
	lines := strings.Split(out, "\n")

	// Primary format: "gpon-onu_1/3/1:1         HWTC00000001        unknown"
	re1 := regexp.MustCompile(`gpon-onu_(\d+/\d+/\d+):(\d+)\s+([A-Za-z0-9]{8,16})\s+`)
	// Alt format: "gpon-olt_1/3/1   1     ZTEG00000001"
	re2 := regexp.MustCompile(`gpon-olt_(\d+/\d+/\d+)\s+(\d+)\s+([A-Za-z0-9]{8,16})`)

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "Onu") || strings.HasPrefix(l, "---") {
			continue
		}
		if m := re1.FindStringSubmatch(l); len(m) >= 4 {
			uncfgs = append(uncfgs, UncfgONU{Port: m[1], OnuID: m[2], SerialNumber: m[3]})
		} else if m := re2.FindStringSubmatch(l); len(m) >= 4 {
			uncfgs = append(uncfgs, UncfgONU{Port: m[1], OnuID: m[2], SerialNumber: m[3]})
		}
	}
	return uncfgs, nil
}

func (z *ZTEC300Driver) GetOpticalPower(onuIntf string) (*OpticalPower, error) {
	if !z.connected {
		if err := z.Connect(); err != nil {
			return nil, errors.New("tidak terhubung ke OLT")
		}
	}

	out, err := z.telnet.ExecCommand("show pon power attenuation "+onuIntf, 8*time.Second)
	if err != nil || out == "" {
		out, _ = z.telnet.ExecCommand("show pon power onu-rx "+onuIntf, 8*time.Second)
	}

	power := &OpticalPower{
		Status: "OFFLINE",
	}

	// Regex for down Rx (ONU Rx)
	downRxRe := regexp.MustCompile(`down[\s\S]*?Rx\s*:\s*([-\d.]+)\s*\(?[Dd][Bb][Mm]\)?`)
	if m := downRxRe.FindStringSubmatch(out); len(m) > 1 {
		power.RxPower = m[1] + " dBm"
		power.OnuRx = m[1] + " dBm"
	} else {
		rxRe := regexp.MustCompile(`([-\d.]+)\s*\(?[Dd][Bb][Mm]\)?`)
		if m := rxRe.FindStringSubmatch(out); len(m) > 1 {
			power.RxPower = m[1] + " dBm"
			power.OnuRx = m[1] + " dBm"
		}
	}

	// Regex for up Rx (OLT Rx)
	upRxRe := regexp.MustCompile(`up[\s\S]*?Rx\s*:\s*([-\d.]+)\s*\(?[Dd][Bb][Mm]\)?`)
	if m := upRxRe.FindStringSubmatch(out); len(m) > 1 {
		power.OltRxPower = m[1] + " dBm"
		power.OltRx = m[1] + " dBm"
	}

	// Regex for up Tx (ONU Tx)
	txRe := regexp.MustCompile(`Tx\s*:\s*([-\d.]+)\s*\(?[Dd][Bb][Mm]\)?`)
	if m := txRe.FindStringSubmatch(out); len(m) > 1 {
		power.TxPower = m[1] + " dBm"
	}

	// Attenuation (avoid matching "(dBm)" on Tx/Rx columns)
	attenRe := regexp.MustCompile(`(?i)down[\s\S]*?([-\d.]+)\s*(?:\(db\)|\bdb\b)`)
	if m := attenRe.FindStringSubmatch(out); len(m) > 1 {
		power.Attenuation = m[1] + " dB"
	}

	// Status determination
	if power.RxPower != "" {
		valStr := strings.TrimSuffix(power.RxPower, " dBm")
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			if val >= -24.0 && val <= -14.0 {
				power.Status = "GOOD"
			} else if val >= -27.0 && val < -24.0 {
				power.Status = "WARNING"
			} else {
				power.Status = "CRITICAL"
			}
		}
	}

	return power, nil
}

func (z *ZTEC300Driver) GetWanIP(onuIntf string) (string, error) {
	if !z.connected {
		return "", errors.New("tidak terhubung ke OLT")
	}
	out, err := z.telnet.ExecCommand("show gpon remote-onu wan-ip "+onuIntf, 8*time.Second)
	if err != nil {
		return "", err
	}
	ipRe := regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`)
	if match := ipRe.FindString(out); match != "" {
		return match, nil
	}
	return "No IP / Connecting", nil
}

func (z *ZTEC300Driver) ProvisionONU(req ProvisionRequest) error {
	if !z.connected {
		return errors.New("tidak terhubung ke OLT")
	}

	cmds := []string{
		"configure terminal",
		fmt.Sprintf("interface gpon-olt_%s", req.Port),
		fmt.Sprintf("onu %s type ALL sn %s", req.OnuID, req.SerialNumber),
		"exit",
		fmt.Sprintf("interface gpon-onu_%s:%s", req.Port, req.OnuID),
	}
	if req.Name != "" {
		cmds = append(cmds, fmt.Sprintf("description \"%s\"", req.Name))
	}
	cmds = append(cmds,
		"tcont 1 profile default",
		"gemport 1 tcont 1",
		fmt.Sprintf("service-port 1 vport 1 user-vlan %s vlan %s", req.VlanID, req.VlanID),
		"exit",
		fmt.Sprintf("pon-onu-mng gpon-onu_%s:%s", req.Port, req.OnuID),
		fmt.Sprintf("service 1 gemport 1 vlan %s", req.VlanID),
		fmt.Sprintf("wan-ip 1 mode pppoe username %s password %s vlan-profile %s host 1", req.Username, req.Password, req.VlanProfile),
	)
	if req.WiFiSSID != "" || req.WiFiPassword != "" {
		if req.WiFiSSID != "" {
			cmds = append(cmds, fmt.Sprintf("wifi-ssid 1 name \"%s\"", req.WiFiSSID))
		}
		if req.WiFiPassword != "" {
			cmds = append(cmds, fmt.Sprintf("wifi-security 1 auth-mode wpa2-psk encrypt-type aes key \"%s\"", req.WiFiPassword))
		}
		cmds = append(cmds, "wifi-switch enable")
	}
	cmds = append(cmds, "exit", "end", "write")

	for _, c := range cmds {
		if _, err := z.telnet.ExecCommand(c, 5*time.Second); err != nil {
			return fmt.Errorf("gagal eksekusi perintah '%s': %w", c, err)
		}
	}
	return nil
}

func (z *ZTEC300Driver) DeleteONU(port string, onuID string) error {
	if !z.connected {
		return errors.New("tidak terhubung ke OLT")
	}
	cmds := []string{"configure terminal", fmt.Sprintf("interface gpon-olt_%s", port), fmt.Sprintf("no onu %s", onuID), "exit", "end", "write"}
	for _, c := range cmds {
		if _, err := z.telnet.ExecCommand(c, 5*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func (z *ZTEC300Driver) RebootONU(onuIntf string) error {
	if !z.connected {
		return errors.New("tidak terhubung ke OLT")
	}
	cmds := []string{"configure terminal", fmt.Sprintf("pon-onu-mng %s", onuIntf), "reboot", "exit", "end"}
	for _, c := range cmds {
		if _, err := z.telnet.ExecCommand(c, 5*time.Second); err != nil {
			return err
		}
	}
	return nil
}

func (z *ZTEC300Driver) ConfigureWiFi(onuIntf, ssid, password string) error {
	if !z.connected {
		return errors.New("tidak terhubung ke OLT")
	}
	cmds := []string{
		"configure terminal",
		fmt.Sprintf("pon-onu-mng %s", onuIntf),
		fmt.Sprintf("wifi-ssid 1 name \"%s\"", ssid),
		fmt.Sprintf("wifi-security 1 auth-mode wpa2-psk encrypt-type aes key \"%s\"", password),
		"wifi-switch enable", "exit", "end", "write",
	}
	for _, c := range cmds {
		if _, err := z.telnet.ExecCommand(c, 5*time.Second); err != nil {
			return err
		}
	}
	return nil
}

// GetFullConfig fetches all live TCONT profiles, Traffic profiles, VLAN profiles, and VLAN summary from the ZTE OLT
func (z *ZTEC300Driver) GetFullConfig() (*OLTFullConfig, error) {
	if !z.connected {
		if err := z.Connect(); err != nil {
			return nil, err
		}
	}

	cfg := &OLTFullConfig{
		Vlans:           make([]VlanItem, 0),
		VlanProfiles:    make([]VlanProfile, 0),
		TcontProfiles:   make([]TcontProfile, 0),
		TrafficProfiles: make([]TrafficProfile, 0),
	}

	// 1. TCONT Profiles
	if out, err := z.telnet.ExecCommand("show gpon profile tcont", 5*time.Second); err == nil {
		lines := strings.Split(out, "\n")
		var currentName string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Profile name :") || strings.HasPrefix(line, "Profile name:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					currentName = strings.TrimSpace(parts[1])
				}
			} else if currentName != "" && (strings.HasPrefix(line, "1 ") || strings.HasPrefix(line, "2 ") || strings.HasPrefix(line, "3 ") || strings.HasPrefix(line, "4 ") || strings.HasPrefix(line, "5 ")) {
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					cfg.TcontProfiles = append(cfg.TcontProfiles, TcontProfile{
						Name: currentName,
						Type: fields[0],
						FBW:  fields[1] + " kbps",
						ABW:  fields[2] + " kbps",
						MBW:  fields[3] + " kbps",
					})
					currentName = ""
				}
			}
		}
	}

	// 2. Traffic Profiles
	if out, err := z.telnet.ExecCommand("show gpon profile traffic", 5*time.Second); err == nil {
		lines := strings.Split(out, "\n")
		var currentName string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Profile name") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					currentName = strings.TrimSpace(parts[1])
				}
			} else if currentName != "" && len(strings.Fields(line)) >= 2 && !strings.Contains(line, "SIR(kbps)") && !strings.Contains(line, "Profile name") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					sir := fields[0]
					pir := fields[1]
					cbs := "default"
					pbs := "default"
					if len(fields) >= 4 {
						cbs = fields[2]
						pbs = fields[3]
					}
					cfg.TrafficProfiles = append(cfg.TrafficProfiles, TrafficProfile{
						Name: currentName,
						SIR:  sir + " kbps",
						PIR:  pir + " kbps",
						CBS:  cbs,
						PBS:  pbs,
					})
					currentName = ""
				}
			}
		}
	}

	// 3. VLAN Profiles
	if out, err := z.telnet.ExecCommand("show gpon onu profile vlan", 5*time.Second); err == nil {
		lines := strings.Split(out, "\n")
		var curProf VlanProfile
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Profile name:") {
				if curProf.Name != "" {
					cfg.VlanProfiles = append(cfg.VlanProfiles, curProf)
					curProf = VlanProfile{}
				}
				curProf.Name = strings.TrimSpace(strings.TrimPrefix(line, "Profile name:"))
			} else if strings.HasPrefix(line, "Tag mode:") {
				curProf.TagMode = strings.TrimSpace(strings.TrimPrefix(line, "Tag mode:"))
			} else if strings.HasPrefix(line, "CVLAN:") {
				curProf.CVLAN = strings.TrimSpace(strings.TrimPrefix(line, "CVLAN:"))
			} else if strings.HasPrefix(line, "CVLAN priority:") {
				curProf.Priority = strings.TrimSpace(strings.TrimPrefix(line, "CVLAN priority:"))
			}
		}
		if curProf.Name != "" {
			cfg.VlanProfiles = append(cfg.VlanProfiles, curProf)
		}
	}

	// 4. VLAN Summary
	if out, err := z.telnet.ExecCommand("show vlan summary", 5*time.Second); err == nil {
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.Contains(line, ",") || strings.Contains(line, "-") {
				// Parse vlan ranges e.g. "1,100,200-202,300"
				parts := strings.Split(line, ",")
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if strings.Contains(p, "-") {
						sub := strings.Split(p, "-")
						if len(sub) == 2 {
							start, _ := strconv.Atoi(strings.TrimSpace(sub[0]))
							end, _ := strconv.Atoi(strings.TrimSpace(sub[1]))
							for v := start; v <= end; v++ {
								cfg.Vlans = append(cfg.Vlans, VlanItem{
									ID:   v,
									Name: fmt.Sprintf("VLAN-%d", v),
									Type: "Smart",
								})
							}
						}
					} else {
						if v, err := strconv.Atoi(p); err == nil {
							cfg.Vlans = append(cfg.Vlans, VlanItem{
								ID:   v,
								Name: fmt.Sprintf("VLAN-%d", v),
								Type: "Smart",
							})
						}
					}
				}
			}
		}
	}

	return cfg, nil
}

// GetONTDetail parses full live diagnostics, WAN IP, PPPoE credentials, and optical power for a specific ONT interface
func (z *ZTEC300Driver) GetONTDetail(onuIntf string) (*ONTDetail, error) {
	if !z.connected {
		if err := z.Connect(); err != nil {
			return nil, errors.New("tidak terhubung ke OLT")
		}
	}

	detail := &ONTDetail{
		Interface:  onuIntf,
		AdminState: "enable",
		PhaseState: "working",
	}

	clean := strings.TrimPrefix(onuIntf, "gpon-onu_")
	parts := strings.Split(clean, ":")
	if len(parts) >= 2 {
		detail.Port = parts[0]
		detail.OnuID = parts[1]
	}

	// 1. show gpon onu detail-info
	if out, err := z.telnet.ExecCommand("show gpon onu detail-info "+onuIntf, 8*time.Second); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Name:") {
				detail.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
			} else if strings.HasPrefix(line, "Type:") {
				detail.ModelType = strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
			} else if strings.HasPrefix(line, "Admin state:") {
				detail.AdminState = strings.TrimSpace(strings.TrimPrefix(line, "Admin state:"))
			} else if strings.HasPrefix(line, "Phase state:") {
				detail.PhaseState = strings.TrimSpace(strings.TrimPrefix(line, "Phase state:"))
			} else if strings.HasPrefix(line, "Serial number:") {
				detail.SerialNumber = strings.TrimSpace(strings.TrimPrefix(line, "Serial number:"))
			} else if strings.HasPrefix(line, "Description:") {
				detail.Description = strings.TrimSpace(strings.TrimPrefix(line, "Description:"))
			} else if strings.HasPrefix(line, "ONU Distance:") {
				detail.Distance = strings.TrimSpace(strings.TrimPrefix(line, "ONU Distance:"))
			} else if strings.HasPrefix(line, "Online Duration:") {
				detail.OnlineDuration = strings.TrimSpace(strings.TrimPrefix(line, "Online Duration:"))
			}
		}
	}

	// 2. show gpon remote-onu wan-ip
	if out, err := z.telnet.ExecCommand("show gpon remote-onu wan-ip "+onuIntf, 8*time.Second); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Username:") {
				detail.PPPoEUser = strings.TrimSpace(strings.TrimPrefix(line, "Username:"))
			} else if strings.HasPrefix(line, "Password:") {
				detail.PPPoEPass = strings.TrimSpace(strings.TrimPrefix(line, "Password:"))
			} else if strings.HasPrefix(line, "Current IP:") {
				ip := strings.TrimSpace(strings.TrimPrefix(line, "Current IP:"))
				if ip != "0.0.0.0" {
					detail.WanIP = ip
				}
			} else if strings.HasPrefix(line, "CVLAN:") {
				detail.CVLAN = strings.TrimSpace(strings.TrimPrefix(line, "CVLAN:"))
			} else if strings.HasPrefix(line, "MAC address:") {
				detail.MACAddress = strings.TrimSpace(strings.TrimPrefix(line, "MAC address:"))
			}
		}
	}

	// 3. show gpon remote-onu wifi / ssid
	if out, err := z.telnet.ExecCommand("show gpon remote-onu ssid "+onuIntf, 5*time.Second); err == nil {
		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimSpace(line)
			if strings.Contains(line, "SSID") && strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
					detail.WiFiSSID = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}

	// 4. Optical Power
	if power, err := z.GetOpticalPower(onuIntf); err == nil {
		detail.OpticalPower = power
	}

	// 5. Update and persist real Serial Number to profile cache so All ONUs & Search are always accurate
	if detail.SerialNumber != "" || detail.Name != "" {
		z.cacheMu.Lock()
		cur := z.profileCache[clean]
		if detail.SerialNumber != "" {
			cur.SN = detail.SerialNumber
		}
		if detail.Name != "" {
			cur.Name = detail.Name
		}
		if detail.PPPoEUser != "" {
			cur.PPPoE = detail.PPPoEUser
		}
		if detail.Description != "" {
			cur.Description = detail.Description
		}
		if detail.ModelType != "" {
			cur.Model = detail.ModelType
		}
		z.profileCache[clean] = cur
		saveDiskCache(z.host, z.profileCache)
		z.cacheMu.Unlock()
	}

	return detail, nil
}

// UpdateONTConfig updates PPPoE credentials, VLAN, and Wi-Fi configuration on the live OLT
func (z *ZTEC300Driver) UpdateONTConfig(onuIntf string, req ONTUpdateRequest) error {
	if !z.connected {
		if err := z.Connect(); err != nil {
			return errors.New("tidak terhubung ke OLT")
		}
	}

	// 1. Update interface description/name, TCONT, and Gemport traffic profile
	if req.Name != "" || req.Description != "" || req.UploadProfile != "" || req.DownloadProfile != "" {
		cmds := []string{
			"configure terminal",
			"interface " + onuIntf,
		}
		if req.Name != "" {
			cmds = append(cmds, fmt.Sprintf("name \"%s\"", req.Name))
		}
		if req.Description != "" {
			cmds = append(cmds, fmt.Sprintf("description \"%s\"", req.Description))
		}
		if req.UploadProfile != "" {
			cmds = append(cmds, fmt.Sprintf("tcont 1 profile %s", req.UploadProfile))
		}
		if req.DownloadProfile != "" {
			cmds = append(cmds, fmt.Sprintf("gemport 1 traffic-limit %s", req.DownloadProfile))
		}
		cmds = append(cmds, "exit")
		for _, c := range cmds {
			_, _ = z.telnet.ExecCommand(c, 3*time.Second)
		}
	}

	// 2. Update pon-onu-mng (PPPoE / VLAN Profile / Wi-Fi)
	mngCmds := []string{
		"configure terminal",
		"pon-onu-mng " + onuIntf,
	}

	if req.PPPoEUser != "" || req.PPPoEPass != "" || req.VlanProfile != "" {
		vlanProf := req.VlanProfile
		if vlanProf == "" {
			vlanProf = "DEFAULT"
		}
		mngCmds = append(mngCmds, fmt.Sprintf("wan-ip 1 mode pppoe username %s password %s vlan-profile %s host 1", req.PPPoEUser, req.PPPoEPass, vlanProf))
	}

	if req.WiFiSSID != "" {
		mngCmds = append(mngCmds, fmt.Sprintf("wifi-ssid 1 name \"%s\"", req.WiFiSSID))
	}
	if req.WiFiPassword != "" {
		mngCmds = append(mngCmds, fmt.Sprintf("wifi-security 1 auth-mode wpa2-psk encrypt-type aes key \"%s\"", req.WiFiPassword))
		mngCmds = append(mngCmds, "wifi-switch enable")
	}

	mngCmds = append(mngCmds, "exit")
	for _, c := range mngCmds {
		_, _ = z.telnet.ExecCommand(c, 4*time.Second)
	}

	return nil
}

// RestoreFactoryONT restores factory settings on the ONT
func (z *ZTEC300Driver) RestoreFactoryONT(onuIntf string) error {
	if !z.connected {
		if err := z.Connect(); err != nil {
			return errors.New("tidak terhubung ke OLT")
		}
	}

	cmds := []string{
		"configure terminal",
		"pon-onu-mng " + onuIntf,
		"restore-factory",
		"exit",
	}
	for _, c := range cmds {
		_, _ = z.telnet.ExecCommand(c, 5*time.Second)
	}
	return nil
}

// SetAdminState sets the administrative state of the ONT (enable / disable)
func (z *ZTEC300Driver) SetAdminState(onuIntf string, state string) error {
	if !z.connected {
		if err := z.Connect(); err != nil {
			return errors.New("tidak terhubung ke OLT")
		}
	}

	state = strings.ToLower(strings.TrimSpace(state))
	if state != "enable" && state != "disable" {
		state = "enable"
	}

	clean := strings.TrimPrefix(onuIntf, "gpon-onu_")
	parts := strings.Split(clean, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid onu interface: %s", onuIntf)
	}
	port := parts[0]
	onuID := parts[1]

	cmds := []string{
		"configure terminal",
		"interface gpon-olt_" + port,
		fmt.Sprintf("onu %s admin-state %s", onuID, state),
		"exit",
	}
	for _, c := range cmds {
		_, _ = z.telnet.ExecCommand(c, 4*time.Second)
	}
	return nil
}

// ExecRawCLI executes an arbitrary raw CLI command on the ZTE OLT and returns the output
func (z *ZTEC300Driver) ExecRawCLI(cmd string) (string, error) {
	if !z.connected {
		if err := z.Connect(); err != nil {
			return "", fmt.Errorf("tidak terhubung ke OLT: %w", err)
		}
	}

	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", nil
	}

	timeout := 10 * time.Second
	lower := strings.ToLower(cmd)
	if strings.Contains(lower, "running-config") || strings.Contains(lower, "show gpon onu state") || strings.Contains(lower, "show vlan") {
		timeout = 30 * time.Second
	}

	out, err := z.telnet.ExecCommand(cmd, timeout)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(out), nil
}

