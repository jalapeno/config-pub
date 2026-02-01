package ingest

import (
	"fmt"
	"strings"

	"github.com/jalapeno/config-pub/internal/gnmi"
)

type MatchInfo struct {
	Hostname string
	RouterID string
	MgmtIP   string
	HasISIS  bool
	HasBGP   bool
	BGPASN   int
}

func ExtractMatchInfo(updates []gnmi.ConfigUpdate) (MatchInfo, error) {
	info := MatchInfo{}

	for _, update := range updates {
		path := strings.TrimSpace(update.Path)
		switch {
		case strings.HasSuffix(path, "/host-names"):
			if name, ok := findString(update.Value, "host-name"); ok {
				info.Hostname = name
			}
		case strings.HasSuffix(path, "/interface-configurations"):
			loopback, mgmt := extractInterfaces(update.Value)
			if loopback != "" {
				info.RouterID = loopback
			}
			if mgmt != "" {
				info.MgmtIP = mgmt
			}
		case strings.HasSuffix(path, "/isis"):
			info.HasISIS = true
		case strings.HasSuffix(path, "/bgp"):
			info.HasBGP = true
			if info.BGPASN == 0 {
				if asn, ok := findASN(update.Value); ok {
					info.BGPASN = asn
				}
			}
		}
	}

	if info.RouterID == "" && info.Hostname == "" {
		return info, fmt.Errorf("missing router_id and hostname in config payload")
	}

	return info, nil
}

func extractInterfaces(value interface{}) (string, string) {
	root, ok := value.(map[string]interface{})
	if !ok {
		return "", ""
	}
	rawList, ok := root["interface-configuration"].([]interface{})
	if !ok {
		return "", ""
	}

	var loopback string
	var mgmt string
	for _, item := range rawList {
		entry, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		ifName, _ := entry["interface-name"].(string)
		ipv4 := extractPrimaryIPv4(entry)
		if ifName == "Loopback0" && ipv4 != "" {
			loopback = ipv4
		}
		if strings.HasPrefix(ifName, "MgmtEth") && ipv4 != "" {
			mgmt = ipv4
		}
	}
	return loopback, mgmt
}

func extractPrimaryIPv4(entry map[string]interface{}) string {
	ipv4, ok := entry["Cisco-IOS-XR-ipv4-io-cfg:ipv4-network"].(map[string]interface{})
	if !ok {
		return ""
	}
	addresses, ok := ipv4["addresses"].(map[string]interface{})
	if !ok {
		return ""
	}
	primary, ok := addresses["primary"].(map[string]interface{})
	if !ok {
		return ""
	}
	address, _ := primary["address"].(string)
	return address
}

func findString(value interface{}, key string) (string, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		if direct, ok := v[key]; ok {
			if str, ok := direct.(string); ok {
				return str, true
			}
		}
		for _, item := range v {
			if str, ok := findString(item, key); ok {
				return str, true
			}
		}
	case []interface{}:
		for _, item := range v {
			if str, ok := findString(item, key); ok {
				return str, true
			}
		}
	}
	return "", false
}

func findASN(value interface{}) (int, bool) {
	switch v := value.(type) {
	case map[string]interface{}:
		if direct, ok := v["four-byte-as"]; ok {
			if asn, ok := findASN(direct); ok {
				return asn, true
			}
		}
		if direct, ok := v["as"]; ok {
			if number, ok := toInt(direct); ok && number > 0 {
				return number, true
			}
		}
		for _, item := range v {
			if asn, ok := findASN(item); ok {
				return asn, true
			}
		}
	case []interface{}:
		for _, item := range v {
			if asn, ok := findASN(item); ok {
				return asn, true
			}
		}
	}
	return 0, false
}

func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}
