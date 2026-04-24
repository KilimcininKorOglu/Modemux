package modem

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type MMCLIController struct{}

func NewMMCLIController() *MMCLIController {
	return &MMCLIController{}
}

func (m *MMCLIController) Detect(ctx context.Context) ([]ModemInfo, error) {
	out, err := m.run(ctx, "mmcli", "-L", "-J")
	if err != nil {
		return nil, fmt.Errorf("listing modems: %w", err)
	}

	var result struct {
		ModemList []string `json:"modem-list"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parsing modem list: %w", err)
	}

	var modems []ModemInfo
	for _, path := range result.ModemList {
		idx := extractModemIndex(path)
		info, err := m.getModemInfo(ctx, idx)
		if err != nil {
			continue
		}
		modems = append(modems, *info)
	}

	return modems, nil
}

func (m *MMCLIController) Status(ctx context.Context, modemIndex int) (*ModemStatus, error) {
	out, err := m.run(ctx, "mmcli", "-m", strconv.Itoa(modemIndex), "-J")
	if err != nil {
		return nil, fmt.Errorf("getting modem %d status: %w", modemIndex, err)
	}

	parsed, err := parseMMCLIModem(out)
	if err != nil {
		return nil, err
	}

	return parsed, nil
}

func (m *MMCLIController) Detail(ctx context.Context, modemIndex int) (*ModemDetail, error) {
	status, err := m.Status(ctx, modemIndex)
	if err != nil {
		return nil, err
	}

	detail := &ModemDetail{
		ModemStatus: *status,
	}

	return detail, nil
}

func (m *MMCLIController) Connect(ctx context.Context, modemIndex int, apn string) error {
	arg := fmt.Sprintf("apn=%s", apn)
	_, err := m.run(ctx, "mmcli", "-m", strconv.Itoa(modemIndex), "--simple-connect="+arg)
	if err != nil {
		return fmt.Errorf("connecting modem %d: %w", modemIndex, err)
	}
	return nil
}

func (m *MMCLIController) Disconnect(ctx context.Context, modemIndex int) error {
	_, err := m.run(ctx, "mmcli", "-m", strconv.Itoa(modemIndex), "--simple-disconnect")
	if err != nil {
		return fmt.Errorf("disconnecting modem %d: %w", modemIndex, err)
	}
	return nil
}

func (m *MMCLIController) SendAT(ctx context.Context, serialPort string, command string) (string, error) {
	_, err := m.run(ctx, "mmcli", "-m", "any", "--command="+command)
	if err != nil {
		return "", fmt.Errorf("sending AT command %q: %w", command, err)
	}
	return "", nil
}

func (m *MMCLIController) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if ok := false; err != nil {
			exitErr, ok = err.(*exec.ExitError)
			if ok {
				return nil, fmt.Errorf("%s %v: %s", name, args, string(exitErr.Stderr))
			}
		}
		return nil, err
	}
	return out, nil
}

func (m *MMCLIController) getModemInfo(ctx context.Context, index int) (*ModemInfo, error) {
	out, err := m.run(ctx, "mmcli", "-m", strconv.Itoa(index), "-J")
	if err != nil {
		return nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	info := &ModemInfo{
		Index: index,
		Path:  fmt.Sprintf("/org/freedesktop/ModemManager1/Modem/%d", index),
	}

	if modemData, ok := raw["modem"].(map[string]interface{}); ok {
		if generic, ok := modemData["generic"].(map[string]interface{}); ok {
			if v, ok := generic["model"].(string); ok {
				info.Model = v
			}
			if v, ok := generic["manufacturer"].(string); ok {
				info.Manufacturer = v
			}
			if v, ok := generic["equipment-identifier"].(string); ok {
				info.IMEI = v
			}
			if ports, ok := generic["ports"].([]interface{}); ok {
				for _, p := range ports {
					ps := fmt.Sprintf("%v", p)
					if strings.Contains(ps, "at") {
						info.SerialPort = extractPortName(ps)
					}
					if strings.Contains(ps, "net") {
						info.DataPort = extractPortName(ps)
					}
				}
			}
		}
	}

	return info, nil
}

func extractModemIndex(path string) int {
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return 0
	}
	idx, _ := strconv.Atoi(parts[len(parts)-1])
	return idx
}

func extractPortName(portStr string) string {
	portStr = strings.TrimSpace(portStr)
	parts := strings.Fields(portStr)
	if len(parts) > 0 {
		return parts[0]
	}
	return portStr
}

func parseMMCLIModem(data []byte) (*ModemStatus, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing modem JSON: %w", err)
	}

	status := &ModemStatus{}

	modemData, ok := raw["modem"].(map[string]interface{})
	if !ok {
		return status, nil
	}

	if generic, ok := modemData["generic"].(map[string]interface{}); ok {
		if v, ok := generic["model"].(string); ok {
			status.Model = v
		}
		if v, ok := generic["manufacturer"].(string); ok {
			status.Manufacturer = v
		}
		if v, ok := generic["equipment-identifier"].(string); ok {
			status.IMEI = v
		}
		if v, ok := generic["state"].(string); ok {
			status.State = parseModemState(v)
		}
		if v, ok := generic["signal-quality"].(map[string]interface{}); ok {
			if val, ok := v["value"].(string); ok {
				status.SignalQuality, _ = strconv.Atoi(val)
			}
		}
		if v, ok := generic["access-technologies"].([]interface{}); ok && len(v) > 0 {
			status.AccessTech = fmt.Sprintf("%v", v[0])
		}
	}

	if threeGPP, ok := modemData["3gpp"].(map[string]interface{}); ok {
		if v, ok := threeGPP["operator-name"].(string); ok {
			status.Operator = v
		}
	}

	if bearer, ok := modemData["bearer"].(map[string]interface{}); ok {
		if ipv4, ok := bearer["ipv4-config"].(map[string]interface{}); ok {
			if v, ok := ipv4["address"].(string); ok {
				status.IP = v
			}
		}
		if v, ok := bearer["interface"].(string); ok {
			status.Interface = v
		}
	}

	return status, nil
}

func parseModemState(s string) State {
	switch strings.ToLower(s) {
	case "connected":
		return StateConnected
	case "connecting":
		return StateConnecting
	case "disabled", "disabling", "enabling", "registered", "searching":
		return StateDisconnected
	case "failed":
		return StateFailed
	default:
		return StateDisconnected
	}
}
