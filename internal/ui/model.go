package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mfayaadh/oltuee/internal/config"
	"github.com/mfayaadh/oltuee/internal/olt"
)

// ViewState represents current active screen
type ViewState int

const (
	StateSessions ViewState = iota
	StateLogin
	StateDashboard
	StateONUList
	StateONUDetail
	StateActivation
	StateHelp
)

// Messages
type cardsLoadedMsg struct{ cards []olt.CardInfo }
type portsLoadedMsg struct{ ports []olt.PONPort }
type onusLoadedMsg struct{ onus []olt.ONUInfo }
type uncfgsLoadedMsg struct{ uncfgs []olt.UncfgONU }
type powerLoadedMsg struct{ power *olt.OpticalPower }
type wanIPLoadedMsg struct{ ip string }
type fullConfigLoadedMsg struct{ config *olt.OLTFullConfig }
type ontDetailLoadedMsg struct{ detail *olt.ONTDetail }
type actionDoneMsg struct{ msg string }
type actionErrMsg struct{ err error }
type liveSummariesLoadedMsg struct{ summaries []olt.SiteSummary }
type connectedMsg struct {
	client  olt.Client
	session config.Session
}
type connectErrMsg struct{ err error }
type disconnectedMsg struct{}

// AppModel is the main Bubble Tea model
type AppModel struct {
	state         ViewState
	store         *config.Store
	oltClient     olt.Client
	activeSession *config.Session
	spinner       spinner.Model
	isLoading     bool
	loadingMsg    string
	statusMsg     string
	width         int
	height        int

	// Sessions view state
	sessionCursor int

	// Add/Edit Login Form state
	formCursor int
	formInputs [8]string // Name, Host, Port, User, Pass, Model, DefaultVLAN, DefaultProfile
	isEditing  bool
	editSessID string

	// Dashboard state
	cards    []olt.CardInfo
	ponPorts []olt.PONPort

	// Unified All ONUs / PON Explorer state
	isSelectingOlt  bool
	oltCursor       int
	selectedCardIdx int
	selectedPortNum int
	isSelectingCard bool
	isSelectingPort bool
	cardCursor      int
	portCursor      int
	onus            []olt.ONUInfo
	onuCursor       int
	onuPage         int
	searchQuery     string
	isSearching     bool
	filterMode      int // 0: All, 1: Online, 2: DyingGasp, 3: LOS
	allCachedONUs   []olt.GlobalONURecord

	// ONT Config & Detail state
	selectedONU    *olt.ONUInfo
	selectedPower  *olt.OpticalPower
	selectedWanIP  string
	ontDetail      *olt.ONTDetail
	oltFullConfig  *olt.OLTFullConfig
	ontEditCursor  int
	ontEditInputs  [10]string // 0:VlanProfile, 1:VlanID, 2:UploadProfile, 3:DownloadProfile, 4:PPPoEUser, 5:PPPoEPass, 6:Name, 7:Description, 8:WiFiSSID, 9:WiFiPass
	ontVlanProfIdx int
	ontVlanIdx     int
	ontTcontIdx    int
	ontTrafficIdx  int

	// Activation Wizard state
	uncfgONUs         []olt.UncfgONU
	uncfgCursor       int
	isActivating      bool
	actFormCursor     int
	actFormInputs     [13]string // 0:OLT, 1:Port, 2:OnuID, 3:SN, 4:Model, 5:VlanProfile, 6:VlanID, 7:Tcont, 8:Traffic, 9:User, 10:Pass, 11:Name, 12:Description
	actOltIdx         int
	actModelIdx       int
	actVlanProfileIdx int
	actVlanIdx        int
	actTcontIdx       int
	actTrafficIdx     int

	liveSummaries []olt.SiteSummary
}

// InitialModel initializes the app model
func InitialModel(store *config.Store) AppModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(BrandEmerald)

	cached := olt.LoadAllCachedONUs(store.Sessions)

	return AppModel{
		state:           StateSessions,
		store:           store,
		spinner:         s,
		sessionCursor:   0,
		selectedCardIdx: 0,
		selectedPortNum: 1,
		portCursor:      1,
		allCachedONUs:   cached,
		formInputs: [8]string{
			"OLT Site Baru", "192.0.2.10", "23", "admin", "", "ZTE-C300", "100", "PPPOE-100",
		},
	}
}

// Init starts spinner and background live metrics sync
func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.syncLiveMetricsCmd())
}

// Update processes Bubble Tea messages
func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case connectedMsg:
		m.isLoading = false
		m.oltClient = msg.client
		m.activeSession = &msg.session
		m.statusMsg = fmt.Sprintf("Terhubung ke %s (%s)", msg.session.Name, msg.session.Host)
		if m.state == StateSessions || m.state == StateLogin {
			m.state = StateDashboard
		}
		return m, tea.Batch(m.loadDashboardData()...)

	case connectErrMsg:
		m.isLoading = false
		m.statusMsg = fmt.Sprintf("Gagal terhubung: %v", msg.err)

	case cardsLoadedMsg:
		m.cards = msg.cards
		gtghs := m.getGTGHCards()
		if len(gtghs) > 0 && m.selectedCardIdx >= len(gtghs) {
			m.selectedCardIdx = 0
		}

	case portsLoadedMsg:
		m.ponPorts = msg.ports

	case fullConfigLoadedMsg:
		m.oltFullConfig = msg.config
		if msg.config != nil {
			// Update default edit inputs if empty
			if len(msg.config.VlanProfiles) > 0 && m.ontEditInputs[0] == "" {
				m.ontEditInputs[0] = msg.config.VlanProfiles[0].Name
				if msg.config.VlanProfiles[0].CVLAN != "" {
					m.ontEditInputs[1] = msg.config.VlanProfiles[0].CVLAN
				}
			}
			if len(msg.config.Vlans) > 0 && m.ontEditInputs[1] == "" {
				m.ontEditInputs[1] = strconv.Itoa(msg.config.Vlans[0].ID)
			}
			// Update activation defaults
			if len(msg.config.VlanProfiles) > 0 && m.actFormInputs[5] == "" {
				m.actFormInputs[5] = msg.config.VlanProfiles[0].Name
				if msg.config.VlanProfiles[0].CVLAN != "" {
					m.actFormInputs[6] = msg.config.VlanProfiles[0].CVLAN
				}
			}
			if len(msg.config.Vlans) > 0 && m.actFormInputs[6] == "" {
				m.actFormInputs[6] = strconv.Itoa(msg.config.Vlans[0].ID)
			}
			if len(msg.config.TcontProfiles) > 0 && m.actFormInputs[7] == "" {
				m.actFormInputs[7] = msg.config.TcontProfiles[0].Name
			}
			if len(msg.config.TrafficProfiles) > 0 && m.actFormInputs[8] == "" {
				m.actFormInputs[8] = msg.config.TrafficProfiles[0].Name
			}
		}

	case ontDetailLoadedMsg:
		m.isLoading = false
		m.ontDetail = msg.detail
		if msg.detail != nil {
			m.ontEditInputs[0] = msg.detail.VlanProfile
			m.ontEditInputs[1] = msg.detail.CVLAN
			m.ontEditInputs[2] = msg.detail.UploadProfile
			m.ontEditInputs[3] = msg.detail.DownloadProfile
			m.ontEditInputs[4] = msg.detail.PPPoEUser
			m.ontEditInputs[5] = msg.detail.PPPoEPass
			m.ontEditInputs[6] = msg.detail.Name
			m.ontEditInputs[7] = msg.detail.Description
			m.ontEditInputs[8] = msg.detail.WiFiSSID
			m.ontEditInputs[9] = msg.detail.WiFiPassword

			if m.ontEditInputs[0] == "" {
				m.ontEditInputs[0] = m.defaultVlanProfile()
			}
			if m.ontEditInputs[1] == "" {
				m.ontEditInputs[1] = m.defaultVlanID()
			}
			if m.ontEditInputs[2] == "" {
				m.ontEditInputs[2] = "default"
			}
			if m.ontEditInputs[3] == "" {
				m.ontEditInputs[3] = "default"
			}

			if msg.detail.OpticalPower != nil {
				m.selectedPower = msg.detail.OpticalPower
			}
			if msg.detail.WanIP != "" {
				m.selectedWanIP = msg.detail.WanIP
			}
		}

	case onusLoadedMsg:
		m.isLoading = false
		m.onus = msg.onus
		m.onuCursor = 0
		m.onuPage = 0
		m.allCachedONUs = olt.LoadAllCachedONUs(m.store.Sessions)

	case uncfgsLoadedMsg:
		m.isLoading = false
		m.uncfgONUs = msg.uncfgs
		m.uncfgCursor = 0

	case powerLoadedMsg:
		m.selectedPower = msg.power

	case wanIPLoadedMsg:
		m.selectedWanIP = msg.ip

	case actionDoneMsg:
		m.isLoading = false
		m.statusMsg = msg.msg

	case actionErrMsg:
		m.isLoading = false
		m.statusMsg = fmt.Sprintf("Error: %v", msg.err)

	case liveSummariesLoadedMsg:
		m.liveSummaries = msg.summaries
		m.allCachedONUs = olt.LoadAllCachedONUs(m.store.Sessions)

	case disconnectedMsg:
		m.isLoading = false
		m.oltClient = nil
		m.activeSession = nil
		m.cards = nil
		m.ponPorts = nil
		m.onus = nil
		m.state = StateSessions
		m.statusMsg = "Terputus dari OLT."

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.Type == tea.KeyCtrlQ {
			if m.oltClient != nil {
				go m.oltClient.Disconnect()
			}
			return m, tea.Quit
		}

		switch m.state {
		case StateSessions:
			return m.handleSessionsKeys(msg)
		case StateLogin:
			return m.handleLoginKeys(msg)
		case StateDashboard:
			return m.handleDashboardKeys(msg)
		case StateONUList:
			return m.handleONUListKeys(msg)
		case StateONUDetail:
			return m.handleONUDetailKeys(msg)
		case StateActivation:
			return m.handleActivationKeys(msg)
		case StateHelp:
			if msg.String() == "esc" || msg.String() == "q" || msg.String() == "1" {
				if m.oltClient != nil && m.oltClient.IsConnected() {
					m.state = StateDashboard
				} else {
					m.state = StateSessions
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) handleSessionsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.sessionCursor > 0 {
			m.sessionCursor--
		}
	case "down", "j":
		if m.sessionCursor < len(m.store.Sessions)-1 {
			m.sessionCursor++
		}
	case "enter":
		if len(m.store.Sessions) > 0 && m.sessionCursor < len(m.store.Sessions) {
			sess := m.store.Sessions[m.sessionCursor]
			m.isLoading = true
			m.loadingMsg = fmt.Sprintf("Menghubungkan ke %s (%s)...", sess.Name, sess.Host)
			return m, m.connectToOLT(sess)
		}
	case "a":
		m.isEditing = false
		m.formCursor = 0
		m.formInputs = [8]string{"OLT Site Baru", "192.0.2.10", "23", "admin", "", "ZTE-C300", "100", "PPPOE-100"}
		m.state = StateLogin
	case "e":
		if len(m.store.Sessions) > 0 && m.sessionCursor < len(m.store.Sessions) {
			sess := m.store.Sessions[m.sessionCursor]
			m.isEditing = true
			m.editSessID = sess.ID
			m.formCursor = 0
			m.formInputs = [8]string{sess.Name, sess.Host, sess.Port, sess.Username, sess.Password, sess.Model, sess.DefaultVLAN, sess.DefaultProfile}
			m.state = StateLogin
		}
	case "d":
		if len(m.store.Sessions) > 0 && m.sessionCursor < len(m.store.Sessions) {
			_ = m.store.Delete(m.store.Sessions[m.sessionCursor].ID)
			if m.sessionCursor >= len(m.store.Sessions) && m.sessionCursor > 0 {
				m.sessionCursor--
			}
			m.statusMsg = "Sesi OLT berhasil dihapus."
		}
	case "2":
		m.state = StateONUList
		m.allCachedONUs = olt.LoadAllCachedONUs(m.store.Sessions)
	case "3":
		m.state = StateActivation
		m.isActivating = false
		return m, m.loadUncfgs()
	case "r":
		m.statusMsg = "Menyinkronkan data seluruh OLT..."
		return m, m.syncLiveMetricsCmd()
	case "?":
		m.state = StateHelp
	}
	return m, nil
}

func (m AppModel) handleLoginKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.state = StateSessions
		return m, nil
	case tea.KeyTab, tea.KeyDown:
		m.formCursor = (m.formCursor + 1) % len(m.formInputs)
	case tea.KeyShiftTab, tea.KeyUp:
		m.formCursor = (m.formCursor - 1 + len(m.formInputs)) % len(m.formInputs)
	case tea.KeyEnter:
		sessID := m.editSessID
		if !m.isEditing || sessID == "" {
			sessID = fmt.Sprintf("olt-%d", time.Now().Unix())
		}
		sess := config.Session{
			ID:             sessID,
			Name:           strings.TrimSpace(m.formInputs[0]),
			Host:           strings.TrimSpace(m.formInputs[1]),
			Port:           strings.TrimSpace(m.formInputs[2]),
			Username:       strings.TrimSpace(m.formInputs[3]),
			Password:       strings.TrimSpace(m.formInputs[4]),
			Model:          strings.TrimSpace(m.formInputs[5]),
			DefaultVLAN:    strings.TrimSpace(m.formInputs[6]),
			DefaultProfile: strings.TrimSpace(m.formInputs[7]),
		}
		_ = m.store.AddOrUpdate(sess)
		m.state = StateSessions
		m.isLoading = true
		m.loadingMsg = fmt.Sprintf("Menghubungkan ke %s...", sess.Name)
		return m, m.connectToOLT(sess)
	case tea.KeyBackspace:
		curr := m.formInputs[m.formCursor]
		if len(curr) > 0 {
			m.formInputs[m.formCursor] = curr[:len(curr)-1]
		}
	case tea.KeyRunes:
		m.formInputs[m.formCursor] += string(msg.Runes)
	}
	return m, nil
}

func (m AppModel) handleDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.state = StateDashboard
	case "2", "/":
		m.state = StateONUList
		m.allCachedONUs = olt.LoadAllCachedONUs(m.store.Sessions)
		if msg.String() == "/" {
			m.isSearching = true
		}
	case "3":
		m.state = StateActivation
		m.isActivating = false
		m.isLoading = true
		m.loadingMsg = "Mencari ONU unconfigured..."
		return m, m.loadUncfgs()
	case "4", "5", "esc":
		if m.oltClient != nil {
			m.isLoading = true
			m.loadingMsg = "Memutuskan koneksi..."
			client := m.oltClient
			return m, func() tea.Msg {
				_ = client.Disconnect()
				return disconnectedMsg{}
			}
		}
		m.state = StateSessions
	case "r":
		m.statusMsg = "Menyinkronkan data live OLT..."
		if m.oltClient != nil && m.oltClient.IsConnected() {
			return m, tea.Batch(append(m.loadDashboardData(), m.syncLiveMetricsCmd())...)
		}
		return m, m.syncLiveMetricsCmd()
	case "?":
		m.state = StateHelp
	}
	return m, nil
}

func (m AppModel) handleONUListKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.isSearching {
		switch msg.Type {
		case tea.KeyEsc:
			m.isSearching = false
			m.searchQuery = ""
			m.onuCursor = 0
			m.onuPage = 0
		case tea.KeyEnter:
			m.isSearching = false
		case tea.KeyBackspace:
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.onuCursor = 0
				m.onuPage = 0
			}
		case tea.KeyRunes:
			m.searchQuery += string(msg.Runes)
			m.onuCursor = 0
			m.onuPage = 0
		}
		return m, nil
	}

	if m.isSelectingOlt {
		switch msg.String() {
		case "up", "k":
			if m.oltCursor > 0 {
				m.oltCursor--
			}
		case "down", "j":
			if m.oltCursor < len(m.store.Sessions) {
				m.oltCursor++
			}
		case "enter":
			m.isSelectingOlt = false
			if m.oltCursor == 0 {
				// All OLTs mode
				m.activeSession = nil
				m.statusMsg = "Menampilkan data seluruh cabang OLT."
			} else {
				sess := m.store.Sessions[m.oltCursor-1]
				m.isLoading = true
				m.loadingMsg = fmt.Sprintf("Menghubungkan ke %s...", sess.Name)
				return m, m.connectToOLT(sess)
			}
		case "esc":
			m.isSelectingOlt = false
		}
		return m, nil
	}

	gtghs := m.getGTGHCards()

	if m.isSelectingCard {
		switch msg.String() {
		case "up", "k":
			if m.cardCursor > 0 {
				m.cardCursor--
			}
		case "down", "j":
			if m.cardCursor < len(gtghs)-1 {
				m.cardCursor++
			}
		case "enter":
			m.isSelectingCard = false
			if len(gtghs) > 0 && m.cardCursor < len(gtghs) {
				m.selectedCardIdx = m.cardCursor
				m.selectedPortNum = 1
				m.portCursor = 1
				portIntf := fmt.Sprintf("1/%s/1", gtghs[m.selectedCardIdx].Slot)
				m.isLoading = true
				m.loadingMsg = fmt.Sprintf("Membuka Slot %s Port 1...", gtghs[m.selectedCardIdx].Slot)
				return m, m.loadONUsForPort(portIntf)
			}
		case "esc":
			m.isSelectingCard = false
		}
		return m, nil
	}

	if m.isSelectingPort {
		switch msg.String() {
		case "up", "k":
			if m.portCursor > 1 {
				m.portCursor--
			}
		case "down", "j":
			if m.portCursor < 16 {
				m.portCursor++
			}
		case "enter":
			m.isSelectingPort = false
			m.selectedPortNum = m.portCursor
			if len(gtghs) > 0 && m.selectedCardIdx < len(gtghs) {
				portIntf := fmt.Sprintf("1/%s/%d", gtghs[m.selectedCardIdx].Slot, m.selectedPortNum)
				m.isLoading = true
				m.loadingMsg = fmt.Sprintf("Membuka port %s...", portIntf)
				return m, m.loadONUsForPort(portIntf)
			}
		case "esc":
			m.isSelectingPort = false
		}
		return m, nil
	}

	filtered := m.getFilteredUnifiedONUs()

	switch msg.String() {
	case "o":
		m.isSelectingOlt = true
		m.oltCursor = 0
		if m.activeSession != nil {
			for i, s := range m.store.Sessions {
				if s.ID == m.activeSession.ID {
					m.oltCursor = i + 1
					break
				}
			}
		}
	case "c":
		gtghs := m.getGTGHCards()
		if len(gtghs) > 0 {
			m.isSelectingCard = true
			m.cardCursor = m.selectedCardIdx
		} else {
			m.statusMsg = "Tidak ada kartu GTGH terdeteksi."
		}
	case "p":
		m.isSelectingPort = true
		m.portCursor = m.selectedPortNum
	case "left", "[", "h":
		if m.activeSession != nil && m.selectedPortNum > 1 {
			m.selectedPortNum--
			m.portCursor = m.selectedPortNum
			if len(gtghs) > 0 && m.selectedCardIdx < len(gtghs) {
				portIntf := fmt.Sprintf("1/%s/%d", gtghs[m.selectedCardIdx].Slot, m.selectedPortNum)
				m.isLoading = true
				m.loadingMsg = fmt.Sprintf("Membuka port %s...", portIntf)
				return m, m.loadONUsForPort(portIntf)
			}
		}
	case "right", "]", "l":
		if m.activeSession != nil && m.selectedPortNum < 16 {
			m.selectedPortNum++
			m.portCursor = m.selectedPortNum
			if len(gtghs) > 0 && m.selectedCardIdx < len(gtghs) {
				portIntf := fmt.Sprintf("1/%s/%d", gtghs[m.selectedCardIdx].Slot, m.selectedPortNum)
				m.isLoading = true
				m.loadingMsg = fmt.Sprintf("Membuka port %s...", portIntf)
				return m, m.loadONUsForPort(portIntf)
			}
		}
	case "f", "tab":
		m.filterMode = (m.filterMode + 1) % 4
		m.onuCursor = 0
		m.onuPage = 0
	case "/":
		m.isSearching = true
	case "up", "k":
		if m.onuCursor > 0 {
			m.onuCursor--
			if m.onuCursor < m.onuPage*12 {
				m.onuPage--
			}
		}
	case "down", "j":
		if m.onuCursor < len(filtered)-1 {
			m.onuCursor++
			if m.onuCursor >= (m.onuPage+1)*12 {
				m.onuPage++
			}
		}
	case "pgup":
		if m.onuPage > 0 {
			m.onuPage--
			m.onuCursor = m.onuPage * 12
		}
	case "pgdown":
		if (m.onuPage+1)*12 < len(filtered) {
			m.onuPage++
			m.onuCursor = m.onuPage * 12
		}
	case "enter":
		if len(filtered) > 0 && m.onuCursor < len(filtered) {
			r := filtered[m.onuCursor]
			targetIntf := r.FullIntf
			if targetIntf == "" {
				targetIntf = fmt.Sprintf("gpon-onu_%s:%s", r.PONPort, r.IDNumber)
			}

			// Find session for this ONU
			var targetSess *config.Session
			for _, s := range m.store.Sessions {
				if s.ID == r.SiteID || s.Host == r.Host {
					targetSess = &s
					break
				}
			}
			if targetSess == nil && m.activeSession != nil {
				targetSess = m.activeSession
			}

			m.selectedONU = &olt.ONUInfo{
				Interface:    targetIntf,
				Port:         r.PONPort,
				OnuID:        r.IDNumber,
				Name:         r.CustomerName,
				Description:  r.Description,
				SerialNumber: r.SN,
				ModelType:    r.ModelType,
				PhaseState:   r.State,
				WanIP:        r.PPPoEUser,
			}
			m.ontEditCursor = 0
			m.ontEditInputs = [10]string{
				m.defaultVlanProfile(), m.defaultVlanID(), "default", "default", r.PPPoEUser, "", r.CustomerName, r.Description, "", "",
			}
			m.state = StateONUDetail
			m.isLoading = true
			m.loadingMsg = fmt.Sprintf("Membaca konfigurasi & telemetri %s...", targetIntf)

			if targetSess != nil && (m.activeSession == nil || m.activeSession.ID != targetSess.ID) {
				m.activeSession = targetSess
				return m, func() tea.Msg {
					driver := olt.NewZTEC300Driver(targetSess.Host, targetSess.Port, targetSess.Username, targetSess.Password)
					_ = driver.Connect()
					m.oltClient = driver
					detail, _ := driver.GetONTDetail(targetIntf)
					cfg, _ := driver.GetFullConfig()
					if detail != nil {
						return ontDetailLoadedMsg{detail: detail}
					}
					if cfg != nil {
						return fullConfigLoadedMsg{config: cfg}
					}
					return actionDoneMsg{msg: "Siap konfigurasi."}
				}
			}

			return m, tea.Batch(
				m.loadONTDetailCmd(targetIntf),
				m.loadFullConfigCmd(),
				m.loadOpticalPower(targetIntf),
			)
		}
	case "1":
		m.state = StateDashboard
	case "3":
		m.state = StateActivation
		m.isActivating = false
		return m, m.loadUncfgs()
	case "4":
		m.state = StateSessions
	}
	return m, nil
}

func (m AppModel) handleONUDetailKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	targetIntf := ""
	if m.selectedONU != nil {
		targetIntf = m.selectedONU.Interface
	} else if m.ontDetail != nil {
		targetIntf = m.ontDetail.Interface
	}

	switch msg.Type {
	case tea.KeyEsc:
		m.state = StateONUList
		return m, nil

	case tea.KeyTab, tea.KeyDown:
		m.ontEditCursor = (m.ontEditCursor + 1) % len(m.ontEditInputs)
		return m, nil

	case tea.KeyShiftTab, tea.KeyUp:
		m.ontEditCursor = (m.ontEditCursor - 1 + len(m.ontEditInputs)) % len(m.ontEditInputs)
		return m, nil

	case tea.KeyLeft, tea.KeyRight, tea.KeySpace:
		// Cycle Dropdowns
		if m.ontEditCursor == 0 { // VLAN Profile Dropdown
			if m.oltFullConfig != nil && len(m.oltFullConfig.VlanProfiles) > 0 {
				dir := 1
				if msg.Type == tea.KeyLeft {
					dir = -1
				}
				m.ontVlanProfIdx = (m.ontVlanProfIdx + dir + len(m.oltFullConfig.VlanProfiles)) % len(m.oltFullConfig.VlanProfiles)
				prof := m.oltFullConfig.VlanProfiles[m.ontVlanProfIdx]
				m.ontEditInputs[0] = prof.Name
				if prof.CVLAN != "" {
					m.ontEditInputs[1] = prof.CVLAN
				}
			} else {
				profs := []string{m.defaultVlanProfile(), "PPPOE-100", "PPPOE-200", "DEFAULT"}
				m.ontVlanProfIdx = (m.ontVlanProfIdx + 1) % len(profs)
				m.ontEditInputs[0] = profs[m.ontVlanProfIdx]
			}
			return m, nil
		} else if m.ontEditCursor == 1 { // CVLAN Dropdown
			if m.oltFullConfig != nil && len(m.oltFullConfig.Vlans) > 0 {
				dir := 1
				if msg.Type == tea.KeyLeft {
					dir = -1
				}
				m.ontVlanIdx = (m.ontVlanIdx + dir + len(m.oltFullConfig.Vlans)) % len(m.oltFullConfig.Vlans)
				m.ontEditInputs[1] = strconv.Itoa(m.oltFullConfig.Vlans[m.ontVlanIdx].ID)
			} else {
				vlans := []string{m.defaultVlanID(), "100", "200", "300", "1"}
				m.ontVlanIdx = (m.ontVlanIdx + 1) % len(vlans)
				m.ontEditInputs[1] = vlans[m.ontVlanIdx]
			}
			return m, nil
		} else if m.ontEditCursor == 2 { // TCONT Dropdown
			if m.oltFullConfig != nil && len(m.oltFullConfig.TcontProfiles) > 0 {
				m.ontTcontIdx = (m.ontTcontIdx + 1) % len(m.oltFullConfig.TcontProfiles)
				m.ontEditInputs[2] = m.oltFullConfig.TcontProfiles[m.ontTcontIdx].Name
			} else {
				tconts := []string{"default", "1G", "UPTO-100M", "UPTO-50M", "UPTO-20M"}
				m.ontTcontIdx = (m.ontTcontIdx + 1) % len(tconts)
				m.ontEditInputs[2] = tconts[m.ontTcontIdx]
			}
			return m, nil
		} else if m.ontEditCursor == 3 { // Traffic Gemport Dropdown
			if m.oltFullConfig != nil && len(m.oltFullConfig.TrafficProfiles) > 0 {
				m.ontTrafficIdx = (m.ontTrafficIdx + 1) % len(m.oltFullConfig.TrafficProfiles)
				m.ontEditInputs[3] = m.oltFullConfig.TrafficProfiles[m.ontTrafficIdx].Name
			} else {
				traffics := []string{"default", "1G", "UPTO-100M", "UPTO-50M", "UPTO-20M"}
				m.ontTrafficIdx = (m.ontTrafficIdx + 1) % len(traffics)
				m.ontEditInputs[3] = traffics[m.ontTrafficIdx]
			}
			return m, nil
		}

	case tea.KeyEnter:
		// Save Configuration
		if targetIntf != "" && m.oltClient != nil {
			req := olt.ONTUpdateRequest{
				VlanProfile:     m.ontEditInputs[0],
				VlanID:          m.ontEditInputs[1],
				UploadProfile:   m.ontEditInputs[2],
				DownloadProfile: m.ontEditInputs[3],
				PPPoEUser:       m.ontEditInputs[4],
				PPPoEPass:       m.ontEditInputs[5],
				Name:            m.ontEditInputs[6],
				Description:     m.ontEditInputs[7],
				WiFiSSID:        m.ontEditInputs[8],
				WiFiPassword:    m.ontEditInputs[9],
			}
			m.isLoading = true
			m.loadingMsg = fmt.Sprintf("Menerapkan konfigurasi ke OLT untuk %s...", targetIntf)
			return m, m.updateONTConfigCmd(targetIntf, req)
		}

	case tea.KeyBackspace:
		if m.ontEditCursor >= 4 {
			curr := m.ontEditInputs[m.ontEditCursor]
			if len(curr) > 0 {
				m.ontEditInputs[m.ontEditCursor] = curr[:len(curr)-1]
			}
		}
		return m, nil

	case tea.KeyRunes:
		if m.ontEditCursor >= 4 {
			m.ontEditInputs[m.ontEditCursor] += string(msg.Runes)
		} else {
			ch := string(msg.Runes)
			if ch == "s" {
				if targetIntf != "" && m.oltClient != nil {
					req := olt.ONTUpdateRequest{
						VlanProfile:     m.ontEditInputs[0],
						VlanID:          m.ontEditInputs[1],
						UploadProfile:   m.ontEditInputs[2],
						DownloadProfile: m.ontEditInputs[3],
						PPPoEUser:       m.ontEditInputs[4],
						PPPoEPass:       m.ontEditInputs[5],
						Name:            m.ontEditInputs[6],
						Description:     m.ontEditInputs[7],
						WiFiSSID:        m.ontEditInputs[8],
						WiFiPassword:    m.ontEditInputs[9],
					}
					m.isLoading = true
					m.loadingMsg = fmt.Sprintf("Menerapkan konfigurasi ke %s...", targetIntf)
					return m, m.updateONTConfigCmd(targetIntf, req)
				}
			} else if ch == "p" {
				m.isLoading = true
				m.loadingMsg = "Mengukur redaman optik live..."
				return m, m.loadOpticalPower(targetIntf)
			} else if ch == "r" {
				m.isLoading = true
				m.loadingMsg = "Mengirim perintah reboot ke ONT..."
				return m, m.rebootONT(targetIntf)
			} else if ch == "f" {
				m.isLoading = true
				m.loadingMsg = "Mereset ONT ke factory default..."
				return m, m.restoreFactoryONT(targetIntf)
			}
		}
	}

	return m, nil
}

func (m AppModel) handleActivationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.isActivating {
		switch msg.String() {
		case "up", "k":
			if m.uncfgCursor > 0 {
				m.uncfgCursor--
			}
		case "down", "j":
			if m.uncfgCursor < len(m.uncfgONUs)-1 {
				m.uncfgCursor++
			}
		case "enter":
			if len(m.uncfgONUs) > 0 && m.uncfgCursor < len(m.uncfgONUs) {
				uncfg := m.uncfgONUs[m.uncfgCursor]
				m.isActivating = true
				m.actFormCursor = 0
				oltName := "OLT Terpilih"
				if m.activeSession != nil {
					oltName = m.activeSession.Name
				}
				m.actFormInputs = [13]string{
					oltName, uncfg.Port, uncfg.OnuID, uncfg.SerialNumber, "ALL",
					m.defaultVlanProfile(), m.defaultVlanID(), "default", "default", "", "", "", "",
				}
				if m.oltClient != nil {
					return m, m.loadFullConfigCmd()
				}
			}
		case "n":
			m.isActivating = true
			m.actFormCursor = 0
			oltName := "OLT Terpilih"
			if m.activeSession != nil {
				oltName = m.activeSession.Name
			}
			m.actFormInputs = [13]string{
				oltName, "1/3/1", "1", "ZTEG00000001", "ALL",
				m.defaultVlanProfile(), m.defaultVlanID(), "default", "default", "", "", "", "",
			}
			if m.oltClient != nil {
				return m, m.loadFullConfigCmd()
			}
		case "r":
			m.isLoading = true
			m.loadingMsg = "Memindai frame unconfigured seluruh OLT..."
			return m, m.loadUncfgs()
		case "1":
			m.state = StateDashboard
		case "2":
			m.state = StateONUList
		case "4":
			m.state = StateSessions
		}
		return m, nil
	}

	// In Activation Form
	switch msg.Type {
	case tea.KeyEsc:
		m.isActivating = false
		return m, nil

	case tea.KeyTab, tea.KeyDown:
		m.actFormCursor = (m.actFormCursor + 1) % len(m.actFormInputs)
		return m, nil

	case tea.KeyShiftTab, tea.KeyUp:
		m.actFormCursor = (m.actFormCursor - 1 + len(m.actFormInputs)) % len(m.actFormInputs)
		return m, nil

	case tea.KeyLeft, tea.KeyRight, tea.KeySpace:
		// Cycle Dropdowns
		if m.actFormCursor == 0 { // OLT Selector
			if len(m.store.Sessions) > 0 {
				m.actOltIdx = (m.actOltIdx + 1) % len(m.store.Sessions)
				sess := m.store.Sessions[m.actOltIdx]
				m.actFormInputs[0] = sess.Name
			}
			return m, nil
		} else if m.actFormCursor == 4 { // Model
			models := []string{"ALL", "ZTE-F609", "ZTE-F660", "ZTE-F670L", "HUAWEI-HG8245H"}
			m.actModelIdx = (m.actModelIdx + 1) % len(models)
			m.actFormInputs[4] = models[m.actModelIdx]
			return m, nil
		} else if m.actFormCursor == 5 { // VLAN Profile
			if m.oltFullConfig != nil && len(m.oltFullConfig.VlanProfiles) > 0 {
				m.actVlanProfileIdx = (m.actVlanProfileIdx + 1) % len(m.oltFullConfig.VlanProfiles)
				prof := m.oltFullConfig.VlanProfiles[m.actVlanProfileIdx]
				m.actFormInputs[5] = prof.Name
				if prof.CVLAN != "" {
					m.actFormInputs[6] = prof.CVLAN
				}
			} else {
				profs := []string{m.defaultVlanProfile(), "PPPOE-100", "PPPOE-200", "DEFAULT"}
				m.actVlanProfileIdx = (m.actVlanProfileIdx + 1) % len(profs)
				m.actFormInputs[5] = profs[m.actVlanProfileIdx]
			}
			return m, nil
		} else if m.actFormCursor == 6 { // VLAN ID
			if m.oltFullConfig != nil && len(m.oltFullConfig.Vlans) > 0 {
				m.actVlanIdx = (m.actVlanIdx + 1) % len(m.oltFullConfig.Vlans)
				m.actFormInputs[6] = strconv.Itoa(m.oltFullConfig.Vlans[m.actVlanIdx].ID)
			} else {
				vlans := []string{m.defaultVlanID(), "100", "200", "300", "1"}
				m.actVlanIdx = (m.actVlanIdx + 1) % len(vlans)
				m.actFormInputs[6] = vlans[m.actVlanIdx]
			}
			return m, nil
		} else if m.actFormCursor == 7 { // TCONT
			if m.oltFullConfig != nil && len(m.oltFullConfig.TcontProfiles) > 0 {
				m.actTcontIdx = (m.actTcontIdx + 1) % len(m.oltFullConfig.TcontProfiles)
				m.actFormInputs[7] = m.oltFullConfig.TcontProfiles[m.actTcontIdx].Name
			} else {
				tconts := []string{"default", "1G", "UPTO-100M", "UPTO-50M", "UPTO-20M"}
				m.actTcontIdx = (m.actTcontIdx + 1) % len(tconts)
				m.actFormInputs[7] = tconts[m.actTcontIdx]
			}
			return m, nil
		} else if m.actFormCursor == 8 { // Traffic Gemport
			if m.oltFullConfig != nil && len(m.oltFullConfig.TrafficProfiles) > 0 {
				m.actTrafficIdx = (m.actTrafficIdx + 1) % len(m.oltFullConfig.TrafficProfiles)
				m.actFormInputs[8] = m.oltFullConfig.TrafficProfiles[m.actTrafficIdx].Name
			} else {
				traffics := []string{"default", "1G", "UPTO-100M", "UPTO-50M", "UPTO-20M"}
				m.actTrafficIdx = (m.actTrafficIdx + 1) % len(traffics)
				m.actFormInputs[8] = traffics[m.actTrafficIdx]
			}
			return m, nil
		}

	case tea.KeyEnter:
		if m.oltClient != nil {
			req := olt.ProvisionRequest{
				Port:         m.actFormInputs[1],
				OnuID:        m.actFormInputs[2],
				SerialNumber: m.actFormInputs[3],
				VlanProfile:  m.actFormInputs[5],
				VlanID:       m.actFormInputs[6],
				Username:     m.actFormInputs[9],
				Password:     m.actFormInputs[10],
				Name:         m.actFormInputs[11],
			}
			m.isLoading = true
			m.loadingMsg = fmt.Sprintf("Mengirim CLI registrasi ONU %s ke OLT...", req.SerialNumber)
			return m, m.provisionONUCmd(req)
		}

	case tea.KeyBackspace:
		if m.actFormCursor != 0 && m.actFormCursor != 4 && m.actFormCursor != 5 && m.actFormCursor != 6 && m.actFormCursor != 7 && m.actFormCursor != 8 {
			curr := m.actFormInputs[m.actFormCursor]
			if len(curr) > 0 {
				m.actFormInputs[m.actFormCursor] = curr[:len(curr)-1]
			}
		}
		return m, nil

	case tea.KeyRunes:
		if m.actFormCursor != 0 && m.actFormCursor != 4 && m.actFormCursor != 5 && m.actFormCursor != 6 && m.actFormCursor != 7 && m.actFormCursor != 8 {
			m.actFormInputs[m.actFormCursor] += string(msg.Runes)
		}
	}

	return m, nil
}

func (m AppModel) defaultVlanID() string {
	if m.activeSession != nil && m.activeSession.DefaultVLAN != "" {
		return m.activeSession.DefaultVLAN
	}
	return "100"
}

func (m AppModel) defaultVlanProfile() string {
	if m.activeSession != nil && m.activeSession.DefaultProfile != "" {
		return m.activeSession.DefaultProfile
	}
	return "PPPOE-100"
}

// Async Commands
func (m AppModel) connectToOLT(sess config.Session) tea.Cmd {
	return func() tea.Msg {
		driver := olt.NewZTEC300Driver(sess.Host, sess.Port, sess.Username, sess.Password)
		if err := driver.Connect(); err != nil {
			return connectErrMsg{err: err}
		}
		return connectedMsg{client: driver, session: sess}
	}
}

func (m AppModel) loadDashboardData() []tea.Cmd {
	return []tea.Cmd{
		m.loadCardsCmd(),
		m.loadPortsCmd(),
		m.loadFullConfigCmd(),
	}
}

func (m AppModel) loadCardsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return cardsLoadedMsg{cards: nil}
		}
		cards, _ := m.oltClient.GetCards()
		return cardsLoadedMsg{cards: cards}
	}
}

func (m AppModel) loadPortsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return portsLoadedMsg{ports: nil}
		}
		ports, _ := m.oltClient.GetPONPorts()
		return portsLoadedMsg{ports: ports}
	}
}

func (m AppModel) loadFullConfigCmd() tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return fullConfigLoadedMsg{config: nil}
		}
		cfg, _ := m.oltClient.GetFullConfig()
		return fullConfigLoadedMsg{config: cfg}
	}
}

func (m AppModel) loadCardsAndDefaultPort() tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return nil
		}
		cards, err := m.oltClient.GetCards()
		if err != nil {
			return actionErrMsg{err: err}
		}
		gtghs := make([]olt.CardInfo, 0)
		for _, c := range cards {
			if strings.Contains(strings.ToLower(c.CfgType), "gtgh") || strings.Contains(strings.ToLower(c.RealType), "gtgh") {
				gtghs = append(gtghs, c)
			}
		}
		if len(gtghs) > 0 {
			portIntf := fmt.Sprintf("1/%s/1", gtghs[0].Slot)
			onus, _ := m.oltClient.GetONUs(portIntf)
			return onusLoadedMsg{onus: onus}
		}
		return cardsLoadedMsg{cards: cards}
	}
}

func (m AppModel) loadONUsForPort(portIntf string) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return nil
		}
		onus, err := m.oltClient.GetONUs(portIntf)
		if err != nil {
			return actionErrMsg{err: err}
		}
		return onusLoadedMsg{onus: onus}
	}
}

func (m AppModel) loadONTDetailCmd(onuIntf string) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return nil
		}
		detail, _ := m.oltClient.GetONTDetail(onuIntf)
		return ontDetailLoadedMsg{detail: detail}
	}
}

func (m AppModel) loadUncfgs() tea.Cmd {
	return func() tea.Msg {
		if m.oltClient != nil {
			uncfgs, _ := m.oltClient.GetUnconfiguredONUs()
			return uncfgsLoadedMsg{uncfgs: uncfgs}
		}
		// Scan across all sessions
		var allUncfgs []olt.UncfgONU
		for _, sess := range m.store.Sessions {
			driver := olt.NewZTEC300Driver(sess.Host, sess.Port, sess.Username, sess.Password)
			if err := driver.Connect(); err == nil {
				if uncfgs, err := driver.GetUnconfiguredONUs(); err == nil {
					allUncfgs = append(allUncfgs, uncfgs...)
				}
				_ = driver.Disconnect()
			}
		}
		return uncfgsLoadedMsg{uncfgs: allUncfgs}
	}
}

func (m AppModel) loadOpticalPower(onuIntf string) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return nil
		}
		power, _ := m.oltClient.GetOpticalPower(onuIntf)
		return powerLoadedMsg{power: power}
	}
}

func (m AppModel) loadWanIP(onuIntf string) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return nil
		}
		ip, _ := m.oltClient.GetWanIP(onuIntf)
		return wanIPLoadedMsg{ip: ip}
	}
}

func (m AppModel) updateONTConfigCmd(onuIntf string, req olt.ONTUpdateRequest) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return actionErrMsg{err: fmt.Errorf("tidak terhubung ke OLT")}
		}
		err := m.oltClient.UpdateONTConfig(onuIntf, req)
		if err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{msg: fmt.Sprintf("Konfigurasi %s berhasil disimpan & diterapkan ke OLT!", onuIntf)}
	}
}

func (m AppModel) rebootONT(onuIntf string) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return nil
		}
		err := m.oltClient.RebootONU(onuIntf)
		if err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{msg: fmt.Sprintf("Perintah reboot berhasil dikirim ke %s!", onuIntf)}
	}
}

func (m AppModel) restoreFactoryONT(onuIntf string) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return nil
		}
		err := m.oltClient.RestoreFactoryONT(onuIntf)
		if err != nil {
			return actionErrMsg{err: err}
		}
		return actionDoneMsg{msg: fmt.Sprintf("ONT %s berhasil direset ke factory default!", onuIntf)}
	}
}

func (m AppModel) provisionONUCmd(req olt.ProvisionRequest) tea.Cmd {
	return func() tea.Msg {
		if m.oltClient == nil {
			return actionErrMsg{err: fmt.Errorf("tidak terhubung ke OLT")}
		}
		err := m.oltClient.ProvisionONU(req)
		if err != nil {
			return actionErrMsg{err: err}
		}
		m.isActivating = false
		return actionDoneMsg{msg: fmt.Sprintf("ONU %s (ID %s) berhasil diaktivasi!", req.SerialNumber, req.OnuID)}
	}
}

func (m AppModel) syncLiveMetricsCmd() tea.Cmd {
	return func() tea.Msg {
		summaries := olt.SyncAllOLTLiveStates(m.store.Sessions)
		return liveSummariesLoadedMsg{summaries: summaries}
	}
}

// View renders the active view
func (m AppModel) View() string {
	if m.isLoading {
		return lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(fmt.Sprintf("%s  %s", m.spinner.View(), lipgloss.NewStyle().Bold(true).Foreground(BrandEmerald).Render(m.loadingMsg)))
	}

	var content string
	switch m.state {
	case StateSessions:
		content = m.RenderSessionsView()
	case StateLogin:
		content = m.RenderLoginView()
	case StateDashboard:
		content = m.RenderDashboard()
	case StateONUList:
		content = m.RenderONUListView()
	case StateONUDetail:
		content = m.RenderONUDetail()
	case StateActivation:
		content = m.RenderActivationView()
	case StateHelp:
		content = m.RenderHelpView()
	default:
		content = m.RenderSessionsView()
	}

	return content
}

