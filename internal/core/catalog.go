package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type NodeCard struct {
	ID       string `json:"id"`
	Remark   string `json:"remark"`
	Proto    string `json:"proto"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Network  string `json:"network"`
	Security string `json:"security"`
	Label    string `json:"label"`
}

func (n *Node) ID() string {
	if n == nil {
		return ""
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{
		n.Proto,
		n.Host,
		strconv.Itoa(n.Port),
		n.UUID,
		n.Password,
		n.Method,
		n.Network,
		n.Security,
		n.SNI,
		n.PBK,
		n.Service,
		n.Path,
	}, "|")))
	return hex.EncodeToString(sum[:8])
}

func (n *Node) Card() NodeCard {
	if n == nil {
		return NodeCard{}
	}
	return NodeCard{
		ID:       n.ID(),
		Remark:   n.Remark,
		Proto:    n.Proto,
		Host:     n.Host,
		Port:     n.Port,
		Network:  n.Network,
		Security: n.Security,
		Label:    n.Label(),
	}
}

func AllShareLines(body string) []string {
	body = expandSubBody(body)
	var out []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(strings.Trim(line, "\"'`"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rejectUnsupported(line) != nil || !isShare(line) {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

func ParseCatalog(source string) ([]NodeCard, []string, string, error) {
	source = CleanSource(source)
	if err := rejectUnsupported(source); err != nil {
		return nil, nil, "", err
	}
	var body, stored string
	switch {
	case isShare(source):
		body = source
		stored = source
	case isHTTPSource(source):
		b, st, err := fetchSubRotate(source)
		if err != nil {
			return nil, nil, "", err
		}
		body = b
		stored = st
	default:
		return nil, nil, "", fmt.Errorf("нужна ссылка подписки или ключ vless, vmess, trojan, ss или hy2")
	}
	var cards []NodeCard
	var lines []string
	for _, line := range AllShareLines(body) {
		n, err := ParseLink(line)
		if err != nil {
			continue
		}
		cards = append(cards, n.Card())
		lines = append(lines, line)
	}
	if len(cards) == 0 {
		if _, err := FirstShareLine(expandSubBody(body)); err != nil {
			return nil, nil, "", err
		}
		return nil, nil, "", fmt.Errorf("в ответе нет ключа vless/vmess/trojan/ss/hy2")
	}
	if strings.TrimSpace(stored) == "" {
		stored = source
	}
	return cards, lines, StripSubHash(stored), nil
}

func pickNode(lines []string, want string) (*Node, error) {
	var first *Node
	for _, line := range lines {
		n, err := ParseLink(line)
		if err != nil {
			continue
		}
		if first == nil {
			first = n
		}
		if want != "" && n.ID() == want {
			return n, nil
		}
	}
	if first == nil {
		return nil, fmt.Errorf("нет узла")
	}
	return first, nil
}

func ResolveChosen(source, want string) (*Node, []NodeCard, string, error) {
	cards, lines, stored, err := ParseCatalog(source)
	if err != nil {
		return nil, nil, "", err
	}
	n, err := pickNode(lines, want)
	if err != nil {
		return nil, nil, "", err
	}
	if strings.TrimSpace(stored) == "" {
		stored = CleanSource(source)
	}
	keepSubURL(stored)
	_ = saveCatalog(StripSubHash(stored), lines)
	setNodes(cards)
	p := LoadPrefs()
	if p.Node != n.ID() {
		p.Node = n.ID()
		_ = SavePrefs(p)
	}
	return n, cards, n.ID(), nil
}

func RefreshCatalog(source string) ([]NodeCard, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = ReadURL()
	}
	if source == "" {
		return nil, fmt.Errorf("ключа нет")
	}
	cards, lines, stored, err := ParseCatalog(source)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(stored) == "" {
		stored = CleanSource(source)
	}
	keepSubURL(stored)
	_ = saveCatalog(StripSubHash(stored), lines)
	setNodes(cards)
	return cards, nil
}

type catalogDisk struct {
	Source string   `json:"source"`
	Lines  []string `json:"lines"`
}

func catalogPath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "nodes.json")
}

func saveCatalog(source string, lines []string) error {
	p := catalogPath()
	if p == "" {
		return fmt.Errorf("нет папки данных")
	}
	b, err := json.Marshal(catalogDisk{Source: source, Lines: lines})
	if err != nil {
		return err
	}
	return writeSecret(p, b)
}

func loadCatalog() (catalogDisk, error) {
	var c catalogDisk
	p := catalogPath()
	if p == "" {
		return c, fmt.Errorf("нет папки данных")
	}
	b, err := readSecret(p)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	return c, nil
}

func cardsFromCatalog() []NodeCard {
	c, err := loadCatalog()
	if err != nil {
		return nil
	}
	var cards []NodeCard
	for _, line := range c.Lines {
		n, err := ParseLink(line)
		if err != nil {
			continue
		}
		cards = append(cards, n.Card())
	}
	return cards
}

var (
	catalogMu    sync.Mutex
	catalogCards []NodeCard
)

func currentNodes() []NodeCard {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if len(catalogCards) == 0 {
		return nil
	}
	out := make([]NodeCard, len(catalogCards))
	copy(out, catalogCards)
	return out
}

func setNodes(cards []NodeCard) {
	catalogMu.Lock()
	catalogCards = cards
	catalogMu.Unlock()
}
