package olt

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/mfayaadh/oltuee/internal/config"
)

// GlobalONURecord represents an aggregated customer ONU record across any OLT site
type GlobalONURecord struct {
	SiteID       string `json:"site_id"`
	SiteName     string `json:"site_name"`
	Host         string `json:"host"`
	Port         string `json:"port"`
	OnuID        string `json:"onu_id"`
	FullIntf     string `json:"full_intf"` // e.g. "gpon-onu_1/3/1:14"
	PONPort      string `json:"pon_port"`  // e.g. "1/3/1"
	IDNumber     string `json:"id_number"` // e.g. "14"
	CustomerName string `json:"customer_name"`
	PPPoEUser    string `json:"pppoe_user"`
	Description  string `json:"description"`
	SN           string `json:"sn"`
	ModelType    string `json:"model_type"`
	State        string `json:"state"` // "working", "dyinggasp", "los", "offline"
	RxPower      string `json:"rx_power"`
}

// SiteSummary represents aggregate metrics for a single OLT site
type SiteSummary struct {
	Session     config.Session `json:"session"`
	OnlineCount int            `json:"online_count"`
	DyingGasp   int            `json:"dying_gasp"`
	LOSCount    int            `json:"los_count"`
	TotalONUs   int            `json:"total_onus"`
	Status      string         `json:"status"` // "ONLINE", "STANDBY", "OFFLINE"
}

// LoadAllCachedONUs loads all customer records across all OLT cache files on disk
func LoadAllCachedONUs(sessions []config.Session) []GlobalONURecord {
	var results []GlobalONURecord

	for _, sess := range sessions {
		data, err := os.ReadFile(getCacheFilePath(sess.Host))
		if err != nil {
			continue
		}

		var cache map[string]ONUProfileCache
		if err := json.Unmarshal(data, &cache); err != nil {
			continue
		}

		for key, item := range cache {
			parts := strings.Split(key, ":")
			ponPort := ""
			idNum := ""
			if len(parts) >= 2 {
				ponPort = parts[0]
				idNum = parts[1]
			} else {
				ponPort = key
			}

			fullIntf := "gpon-onu_" + key
			rec := GlobalONURecord{
				SiteID:       sess.ID,
				SiteName:     sess.Name,
				Host:         sess.Host,
				Port:         sess.Port,
				OnuID:        key,
				FullIntf:     fullIntf,
				PONPort:      ponPort,
				IDNumber:     idNum,
				CustomerName: item.Name,
				PPPoEUser:    item.PPPoE,
				Description:  item.Description,
				SN:           item.SN,
				ModelType:    item.Model,
				State:        item.State,
				RxPower:      item.RxPower,
			}
			if rec.State == "" {
				rec.State = "working"
			}
			if rec.SN == "" {
				rec.SN = "-"
			}
			if rec.ModelType == "" {
				rec.ModelType = "-"
			}
			if rec.Description == "" {
				rec.Description = fmt.Sprintf("PON-%s/%s", ponPort, idNum)
			}
			if rec.RxPower == "" {
				rec.RxPower = "-"
			}
			results = append(results, rec)
		}
	}

	return results
}

// SearchGlobalONUs performs case-insensitive multi-field search and status filtering
func SearchGlobalONUs(records []GlobalONURecord, query string, filterMode int) []GlobalONURecord {
	q := strings.ToLower(strings.TrimSpace(query))
	var filtered []GlobalONURecord

	for _, r := range records {
		if filterMode == 1 && !strings.EqualFold(r.State, "working") {
			continue
		}
		if filterMode == 2 && !strings.EqualFold(r.State, "dyinggasp") && !strings.EqualFold(r.State, "dying-gasp") {
			continue
		}
		if filterMode == 3 && !strings.EqualFold(r.State, "los") && !strings.EqualFold(r.State, "loss-of-signal") {
			continue
		}

		if q == "" {
			filtered = append(filtered, r)
			continue
		}

		tokens := strings.Fields(q)
		allMatched := true
		searchStr := strings.ToLower(fmt.Sprintf("%s %s %s %s %s %s %s %s",
			r.CustomerName, r.PPPoEUser, r.Description, r.SN, r.FullIntf, r.OnuID, r.SiteName, r.ModelType))
		snClean := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(r.SN, ""))

		for _, token := range tokens {
			cleanToken := regexp.MustCompile(`[^a-zA-Z0-9]`).ReplaceAllString(token, "")
			if !strings.Contains(searchStr, token) && (cleanToken == "" || !strings.Contains(snClean, cleanToken)) {
				allMatched = false
				break
			}
		}

		if allMatched {
			filtered = append(filtered, r)
		}
	}

	return filtered
}

// CalculateSiteSummaries calculates total ONUs, Online, DyingGasp, and LOS per site from cache
func CalculateSiteSummaries(sessions []config.Session, records []GlobalONURecord) []SiteSummary {
	var summaries []SiteSummary

	counts := make(map[string]*SiteSummary)
	for _, sess := range sessions {
		s := &SiteSummary{
			Session: sess,
			Status:  "ONLINE",
		}
		counts[sess.ID] = s
	}

	for _, r := range records {
		s, ok := counts[r.SiteID]
		if !ok {
			continue
		}
		s.TotalONUs++
		state := strings.ToLower(r.State)
		if state == "working" || state == "online" || state == "up" || state == "" {
			s.OnlineCount++
		} else if strings.Contains(state, "dying") || strings.Contains(state, "gasp") {
			s.DyingGasp++
		} else if strings.Contains(state, "los") || strings.Contains(state, "off") {
			s.LOSCount++
		}
	}

	for _, sess := range sessions {
		if s, ok := counts[sess.ID]; ok {
			summaries = append(summaries, *s)
		}
	}

	return summaries
}

// SyncAllOLTLiveStates concurrently queries show gpon onu state on all OLTs in parallel
func SyncAllOLTLiveStates(sessions []config.Session) []SiteSummary {
	var wg sync.WaitGroup
	var mu sync.Mutex
	summaries := make(map[string]*SiteSummary)

	pattern := regexp.MustCompile(`(\d+/\d+/\d+:\d+)\s+(\S+)\s+(\S+)\s+(\S+)`)

	for _, sess := range sessions {
		s := &SiteSummary{
			Session: sess,
			Status:  "ONLINE",
		}
		summaries[sess.ID] = s

		wg.Add(1)
		go func(sConf config.Session) {
			defer wg.Done()
			tn, err := NewTelnetClient(sConf.Host, sConf.Port, 5*time.Second)
			if err != nil {
				mu.Lock()
				summaries[sConf.ID].Status = "OFFLINE"
				mu.Unlock()
				return
			}
			defer tn.Close()

			out, err := tn.ReadUntilLogin("Username:", "username:", "login:", "#", ">")
			if err == nil && (strings.Contains(strings.ToLower(out), "sername") || strings.Contains(strings.ToLower(out), "login")) {
				_ = tn.sendLine(sConf.Username)
				_, _ = tn.ReadUntilLogin("Password:", "password:")
				_ = tn.sendLine(sConf.Password)
			}
			_ = tn.readRaw(1*time.Second, 5*time.Second, func(s string) bool { return tn.looksLikePromptAtEnd(s) })
			_, _ = tn.ExecCommand("terminal length 0")

			stateOut, err := tn.ExecCommand("show gpon onu state", 15*time.Second)
			if err != nil || stateOut == "" {
				return
			}

			cache := loadDiskCache(sConf.Host)
			working, dyinggasp, los, total := 0, 0, 0, 0
			activePorts := make(map[string]bool)

			for _, line := range strings.Split(stateOut, "\n") {
				m := pattern.FindStringSubmatch(strings.TrimSpace(line))
				if len(m) >= 5 {
					onuKey := m[1]
					state := m[4]
					total++

					parts := strings.Split(onuKey, ":")
					if len(parts) >= 1 {
						activePorts[parts[0]] = true
					}

					stLower := strings.ToLower(state)
					if strings.Contains(stLower, "working") || strings.Contains(stLower, "online") {
						working++
					} else if strings.Contains(stLower, "dying") || strings.Contains(stLower, "gasp") {
						dyinggasp++
					} else if strings.Contains(stLower, "los") {
						los++
					}

					item, exists := cache[onuKey]
					if !exists {
						item = ONUProfileCache{}
					}
					item.State = state
					cache[onuKey] = item
				}
			}

			// 2. Query real hardware Serial Numbers per active port via `show gpon onu baseinfo gpon-olt_X/Y/Z`
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
							item, exists := cache[onuKey]
							if !exists {
								item = ONUProfileCache{}
							}
							item.SN = sn
							item.Model = model
							cache[onuKey] = item
						}
					}
				}
			}

			saveDiskCache(sConf.Host, cache)

			mu.Lock()
			summaries[sConf.ID].TotalONUs = total
			summaries[sConf.ID].OnlineCount = working
			summaries[sConf.ID].DyingGasp = dyinggasp
			summaries[sConf.ID].LOSCount = los
			summaries[sConf.ID].Status = "ONLINE"
			mu.Unlock()
		}(sess)
	}

	wg.Wait()

	var list []SiteSummary
	for _, sess := range sessions {
		if s, ok := summaries[sess.ID]; ok {
			list = append(list, *s)
		}
	}
	return list
}
