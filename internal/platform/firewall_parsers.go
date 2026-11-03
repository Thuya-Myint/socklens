package platform

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Thuya-Myint/socklens/internal/firewall"
)

func rawPolicyRule(source, id, direction, action, family, raw string) firewall.Rule {
	return firewall.Rule{Source: source, ID: id, Name: id, Direction: direction, Action: action, Family: family, Protocols: []string{}, LocalPorts: []string{}, RemotePorts: []string{}, LocalAddresses: []string{}, RemoteAddresses: []string{}, Conditions: []string{"Original conditions and rule traversal must be reviewed; no final verdict is inferred."}, Raw: raw}
}
func decodeNFT(data []byte, s *firewall.Snapshot) error {
	var doc struct {
		Items []map[string]json.RawMessage `json:"nftables"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Items == nil {
		return fmt.Errorf("missing nftables array")
	}
	type chain struct {
		Family string
		Table  string
		Name   string
		Hook   string
		Policy string
	}
	chains := map[string]chain{}
	profiles := []firewall.Profile{}
	rules := []firewall.Rule{}
	key := func(family, table, name string) string { return family + "/" + table + "/" + name }
	for _, item := range doc.Items {
		if data, ok := item["chain"]; ok {
			var c chain
			if err := json.Unmarshal(data, &c); err != nil {
				return err
			}
			chains[key(c.Family, c.Table, c.Name)] = c
			if c.Hook == "input" || c.Hook == "output" {
				profile := firewall.Profile{Name: key(c.Family, c.Table, c.Name), Enabled: "chain observed", Inbound: "not applicable", Outbound: "not applicable", Detail: "Hook=" + c.Hook + "; chain policy=" + c.Policy + ". Other chains and earlier/later hooks still apply."}
				if c.Hook == "input" {
					profile.Inbound = c.Policy
				} else {
					profile.Outbound = c.Policy
				}
				profiles = append(profiles, profile)
			}
		}
	}
	for _, item := range doc.Items {
		if data, ok := item["rule"]; ok {
			var row struct {
				Family string
				Table  string
				Chain  string
				Handle uint64
				Expr   []map[string]json.RawMessage
			}
			if err := json.Unmarshal(data, &row); err != nil {
				return err
			}
			c := chains[key(row.Family, row.Table, row.Chain)]
			direction := "unknown"
			if c.Hook == "input" {
				direction = "inbound"
			}
			if c.Hook == "output" {
				direction = "outbound"
			}
			family := "any"
			if row.Family == "ip" {
				family = "ipv4"
			}
			if row.Family == "ip6" {
				family = "ipv6"
			}
			r := rawPolicyRule("nftables", fmt.Sprintf("%s/%d", key(row.Family, row.Table, row.Chain), row.Handle), direction, "unknown", family, string(data))
			for _, expr := range row.Expr {
				if _, ok := expr["accept"]; ok {
					r.Action = "allow"
				}
				if _, ok := expr["drop"]; ok {
					r.Action = "block"
				}
				if _, ok := expr["reject"]; ok {
					r.Action = "block"
				}
				if match, ok := expr["match"]; ok && direction != "unknown" {
					var m struct {
						Op   string
						Left struct {
							Payload struct {
								Protocol string
								Field    string
							}
							Meta struct{ Key string }
						}
						Right any
					}
					if json.Unmarshal(match, &m) != nil {
						return fmt.Errorf("invalid nftables match")
					}
					if m.Op != "==" && m.Op != "in" {
						continue
					}
					values, ok := nftValues(m.Right)
					if !ok {
						continue
					}
					payload := m.Left.Payload
					if m.Left.Meta.Key == "l4proto" {
						r.Protocols = append(r.Protocols, values...)
					}
					if payload.Protocol == "tcp" || payload.Protocol == "udp" {
						r.Protocols = append(r.Protocols, payload.Protocol)
						if payload.Field == "dport" {
							if direction == "inbound" {
								r.LocalPorts = values
							} else {
								r.RemotePorts = values
							}
						}
						if payload.Field == "sport" {
							if direction == "inbound" {
								r.RemotePorts = values
							} else {
								r.LocalPorts = values
							}
						}
					}
					if payload.Protocol == "ip" || payload.Protocol == "ip6" {
						if payload.Protocol == "ip" {
							r.Family = "ipv4"
						} else {
							r.Family = "ipv6"
						}
						if payload.Field == "saddr" {
							if direction == "inbound" {
								r.RemoteAddresses = values
							} else {
								r.LocalAddresses = values
							}
						}
						if payload.Field == "daddr" {
							if direction == "inbound" {
								r.LocalAddresses = values
							} else {
								r.RemoteAddresses = values
							}
						}
					}
				}
			}
			rules = append(rules, r)
		}
	}
	s.Rules = append(s.Rules, rules...)
	s.Profiles = append(s.Profiles, profiles...)
	return nil
}
func nftValues(value any) ([]string, bool) {
	switch v := value.(type) {
	case string:
		return []string{v}, true
	case float64:
		if v < 0 || v > 65535 || v != float64(uint64(v)) {
			return nil, false
		}
		return []string{strconv.FormatUint(uint64(v), 10)}, true
	case map[string]any:
		if values, ok := v["set"].([]any); ok {
			result := []string{}
			for _, item := range values {
				parts, ok := nftValues(item)
				if !ok {
					return nil, false
				}
				result = append(result, parts...)
			}
			return result, true
		}
		if values, ok := v["range"].([]any); ok && len(values) == 2 {
			lo, a := nftValues(values[0])
			hi, b := nftValues(values[1])
			if a && b && len(lo) == 1 && len(hi) == 1 {
				return []string{lo[0] + "-" + hi[0]}, true
			}
		}
		if prefix, ok := v["prefix"].(map[string]any); ok {
			addr, a := prefix["addr"].(string)
			length, b := prefix["len"].(float64)
			if a && b && length >= 0 && length <= 128 && length == float64(int(length)) {
				return []string{addr + "/" + strconv.Itoa(int(length))}, true
			}
		}
	}
	return nil, false
}
func decodeIPTables(data []byte, source string, s *firewall.Snapshot) {
	family := "ipv4"
	if source == "ip6tables-save" {
		family = "ipv6"
	}
	for number, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ":INPUT ") || strings.HasPrefix(line, ":OUTPUT ") {
			fields := strings.Fields(line)
			policy := "unknown"
			if fields[1] == "ACCEPT" {
				policy = "allow"
			}
			if fields[1] == "DROP" {
				policy = "block"
			}
			profile := firewall.Profile{Name: source + " " + strings.TrimPrefix(fields[0], ":"), Enabled: "chain observed", Inbound: "not applicable", Outbound: "not applicable", Detail: "Filter-table base-chain policy; other chains/hooks and state tracking still apply."}
			if fields[0] == ":INPUT" {
				profile.Inbound = policy
			} else {
				profile.Outbound = policy
			}
			s.Profiles = append(s.Profiles, profile)
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "-A" {
			continue
		}
		direction := "unknown"
		if fields[1] == "INPUT" {
			direction = "inbound"
		}
		if fields[1] == "OUTPUT" {
			direction = "outbound"
		}
		r := rawPolicyRule(source, strconv.Itoa(number+1), direction, "unknown", family, line)
		simple := !strings.Contains(line, "!") && !strings.ContainsAny(line, "\"'\\")
		for i := 2; i+1 < len(fields); i++ {
			value := fields[i+1]
			if fields[i] == "-j" {
				switch value {
				case "ACCEPT":
					r.Action = "allow"
				case "DROP", "REJECT":
					r.Action = "block"
				}
			}
			if !simple || direction == "unknown" {
				continue
			}
			switch fields[i] {
			case "-p", "--protocol":
				r.Protocols = []string{value}
			case "-s", "--source":
				if direction == "inbound" {
					r.RemoteAddresses = []string{value}
				} else {
					r.LocalAddresses = []string{value}
				}
			case "-d", "--destination":
				if direction == "inbound" {
					r.LocalAddresses = []string{value}
				} else {
					r.RemoteAddresses = []string{value}
				}
			case "--dport", "--dports", "--destination-port":
				values := strings.Split(strings.ReplaceAll(value, ":", "-"), ",")
				if direction == "inbound" {
					r.LocalPorts = values
				} else {
					r.RemotePorts = values
				}
			case "--sport", "--sports", "--source-port":
				values := strings.Split(strings.ReplaceAll(value, ":", "-"), ",")
				if direction == "inbound" {
					r.RemotePorts = values
				} else {
					r.LocalPorts = values
				}
			case "-i", "--in-interface", "-o", "--out-interface":
				r.Interfaces += fields[i] + "=" + value + " "
			}
		}
		s.Rules = append(s.Rules, r)
	}
}
func decodePF(data []byte, s *firewall.Snapshot) {
	for i, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		action := "unknown"
		switch fields[0] {
		case "pass":
			action = "allow"
		case "block":
			action = "block"
		case "anchor":
		default:
			continue
		}
		direction := "both"
		header := strings.SplitN(line, " from ", 2)[0]
		for _, f := range strings.Fields(header) {
			if f == "in" {
				direction = "inbound"
				break
			}
			if f == "out" {
				direction = "outbound"
				break
			}
		}

		// Preserve PF syntax verbatim. Do not misread anchors, macros, tables,
		// negation, quick/last-match precedence, or service-name port expressions.
		r := rawPolicyRule("macos-pf", strconv.Itoa(i+1), direction, action, "any", line)
		r.Conditions = append(r.Conditions, "PF fields are preserved as raw policy; port, IP, interface, anchor and state restrictions are unresolved.")
		s.Rules = append(s.Rules, r)
	}
}

var alfAppLine = regexp.MustCompile(`^\s*[0-9]+\s*:\s*(/.+?)\s*$`)

func decodeALF(data []byte, s *firewall.Snapshot) {
	program := ""
	for i, line := range strings.Split(string(data), "\n") {
		if match := alfAppLine.FindStringSubmatch(line); match != nil {
			program = match[1]
			continue
		}
		if program == "" {
			continue
		}
		action := "unknown"
		if strings.Contains(line, "Allow incoming connections") {
			action = "allow"
		}
		if strings.Contains(line, "Block incoming connections") {
			action = "block"
		}
		if action == "unknown" {
			continue
		}
		r := rawPolicyRule("macos-application-firewall", strconv.Itoa(i+1), "inbound", action, "any", line)
		r.Program = program
		r.Name = program
		r.Conditions = append(r.Conditions, "Application Firewall enabled/block-all state and signed-app exemptions must also be considered.")
		s.Rules = append(s.Rules, r)
		program = ""
	}
}
