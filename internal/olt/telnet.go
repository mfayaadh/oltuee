package olt

import (
	"bytes"
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

// TelnetClient handles raw socket connection and Telnet protocol handshakes.
// All public methods are goroutine-safe via internal mutex.
type TelnetClient struct {
	conn     net.Conn
	mu       sync.Mutex
	hostname string
	timeout  time.Duration
}

func NewTelnetClient(host, port string, timeout time.Duration) (*TelnetClient, error) {
	address := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke %s: %w", address, err)
	}
	return &TelnetClient{conn: conn, timeout: timeout}, nil
}

func (t *TelnetClient) SetHostname(h string) { t.hostname = h }
func (t *TelnetClient) Close() error {
	if t.conn != nil {
		_ = t.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
		_, _ = t.conn.Write([]byte("exit\r\n"))
		return t.conn.Close()
	}
	return nil
}

func (t *TelnetClient) readRaw(gap time.Duration, maxWait time.Duration, done func(string) bool) string {
	var result bytes.Buffer
	buf := make([]byte, 8192)
	deadline := time.Now().Add(maxWait)

	for time.Now().Before(deadline) {
		_ = t.conn.SetReadDeadline(time.Now().Add(gap))
		n, err := t.conn.Read(buf)
		if n > 0 {
			filtered := t.filterTelnetIAC(buf[:n])
			result.Write(filtered)
			text := result.String()

			if strings.HasSuffix(strings.TrimRight(text, " \r\n"), "--More--") {
				_, _ = t.conn.Write([]byte(" "))
				cleaned := strings.ReplaceAll(text, "  --More--", "")
				cleaned = strings.ReplaceAll(cleaned, "--More--", "")
				result.Reset()
				result.WriteString(cleaned)
				continue
			}

			if done != nil && done(text) {
				return result.String()
			}
			deadline = time.Now().Add(maxWait)
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if result.Len() > 0 {
					text := result.String()
					if t.looksLikePromptAtEnd(text) {
						return text
					}
					if done != nil && done(text) {
						return text
					}
				}
				continue
			}
			break
		}
	}
	return result.String()
}

func (t *TelnetClient) looksLikePromptAtEnd(text string) bool {
	return t.looksLikeCommandFinished(text, "")
}

var promptRegex = regexp.MustCompile(`^[A-Za-z0-9\-_()./:]+[#>]\s*$`)

// looksLikeCommandFinished checks if the accumulated text contains the complete command output
// ending with a fresh CLI prompt line.
func (t *TelnetClient) looksLikeCommandFinished(text, cmd string) bool {
	trimmed := strings.TrimRight(text, " \r\n\x00")
	if len(trimmed) == 0 {
		return false
	}
	lastChar := trimmed[len(trimmed)-1]
	if lastChar != '#' && lastChar != '>' {
		return false
	}
	lastNL := strings.LastIndexAny(trimmed, "\n")
	var lastLine string
	if lastNL >= 0 {
		lastLine = strings.TrimSpace(trimmed[lastNL+1:])
	} else {
		if cmd != "" {
			return false
		}
		lastLine = strings.TrimSpace(trimmed)
	}
	if len(lastLine) == 0 || len(lastLine) > 80 {
		return false
	}
	if cmd != "" && strings.Contains(lastLine, cmd) {
		return false
	}
	if strings.Contains(lastLine, "---") || strings.Contains(lastLine, "===") {
		return false
	}
	return promptRegex.MatchString(lastLine)
}

func (t *TelnetClient) ReadUntilLogin(delims ...string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Pulse newline to wake up quiet OLT banners
	_ = t.conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	_, _ = t.conn.Write([]byte("\r\n"))

	text := t.readRaw(500*time.Millisecond, 8*time.Second, func(s string) bool {
		lower := strings.ToLower(s)
		for _, d := range delims {
			if strings.Contains(lower, strings.ToLower(d)) {
				return true
			}
		}
		return false
	})
	if len(text) == 0 {
		// Second pulse attempt
		_, _ = t.conn.Write([]byte("\r\n"))
		text = t.readRaw(500*time.Millisecond, 4*time.Second, func(s string) bool {
			lower := strings.ToLower(s)
			for _, d := range delims {
				if strings.Contains(lower, strings.ToLower(d)) {
					return true
				}
			}
			return false
		})
	}
	if len(text) == 0 {
		return "", fmt.Errorf("timeout menunggu prompt login")
	}
	return text, nil
}

func (t *TelnetClient) sendLine(cmd string) error {
	_ = t.conn.SetWriteDeadline(time.Now().Add(t.timeout))
	_, err := t.conn.Write([]byte(cmd + "\r\n"))
	return err
}

func (t *TelnetClient) ExecCommand(cmd string, extraWait ...time.Duration) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.drainBuffer()

	if err := t.sendLine(cmd); err != nil {
		return "", fmt.Errorf("gagal mengirim perintah '%s': %w", cmd, err)
	}

	maxWait := 10 * time.Second
	if len(extraWait) > 0 && extraWait[0] > 0 {
		maxWait = extraWait[0]
	}

	raw := t.readRaw(1500*time.Millisecond, maxWait, func(s string) bool {
		return t.looksLikeCommandFinished(s, cmd)
	})

	return t.cleanCommandOutput(raw, cmd), nil
}

func (t *TelnetClient) drainBuffer() {
	buf := make([]byte, 4096)
	for {
		_ = t.conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, err := t.conn.Read(buf)
		if n > 0 {
			_ = t.filterTelnetIAC(buf[:n])
		}
		if err != nil {
			break
		}
	}
}

func (t *TelnetClient) cleanCommandOutput(raw, cmd string) string {
	lines := strings.Split(raw, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimRight(line, "\r \x00")
		if len(cleaned) == 0 && line == "" {
			continue
		}
		if strings.Contains(line, cmd) {
			continue
		}
		if t.hostname != "" && strings.HasPrefix(line, t.hostname) &&
			(strings.HasSuffix(line, "#") || strings.HasSuffix(line, ">")) {
			continue
		}
		cleaned = append(cleaned, line)
	}
	for len(cleaned) > 0 && strings.TrimSpace(cleaned[len(cleaned)-1]) == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return strings.Join(cleaned, "\n")
}

func (t *TelnetClient) filterTelnetIAC(data []byte) []byte {
	var clean []byte
	i := 0
	for i < len(data) {
		if data[i] == 0xFF {
			if i+1 < len(data) {
				cmd := data[i+1]
				if cmd >= 251 && cmd <= 254 {
					if i+2 < len(data) {
						opt := data[i+2]
						switch cmd {
						case 253:
							_, _ = t.conn.Write([]byte{0xFF, 252, opt})
						case 251:
							_, _ = t.conn.Write([]byte{0xFF, 254, opt})
						}
						i += 3
						continue
					}
				} else if cmd == 0xFF {
					clean = append(clean, 0xFF)
					i += 2
					continue
				}
			}
			i += 2
			continue
		}
		clean = append(clean, data[i])
		i++
	}
	return clean
}
