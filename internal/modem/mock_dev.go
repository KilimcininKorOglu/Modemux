//go:build dev

package modem

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

type mockController struct {
	mu         sync.RWMutex
	modemCount int
	modems     map[int]*mockModem
}

type mockModem struct {
	info      ModemInfo
	state     State
	ip        string
	operator  string
	signal    int
	connected time.Time
	rotations int
}

func NewMockController(modemCount int) Controller {
	mc := &mockController{
		modemCount: modemCount,
		modems:     make(map[int]*mockModem),
	}

	operators := []string{"Turkcell", "Vodafone", "Türk Telekom"}
	models := []string{"Huawei E3372h", "ZTE MF833V", "Huawei E3531"}

	for i := 0; i < modemCount; i++ {
		mc.modems[i] = &mockModem{
			info: ModemInfo{
				Index:        i,
				Path:         fmt.Sprintf("/org/freedesktop/ModemManager1/Modem/%d", i),
				Model:        models[i%len(models)],
				Manufacturer: "Mock",
				SerialPort:   fmt.Sprintf("/dev/ttyUSB%d", i*2),
				DataPort:     fmt.Sprintf("wwan%d", i),
				IMEI:         strings.Repeat(strconv.Itoa(i+1), 15)[:15],
			},
			state:     StateConnected,
			ip:        randomIPForModem(i),
			operator:  operators[i%len(operators)],
			signal:    60 + rand.Intn(35),
			connected: time.Now(),
		}
	}

	return mc
}

func (mc *mockController) Detect(_ context.Context) ([]ModemInfo, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	var infos []ModemInfo
	for _, m := range mc.modems {
		infos = append(infos, m.info)
	}
	return infos, nil
}

func (mc *mockController) Status(_ context.Context, modemIndex int) (*ModemStatus, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	m, ok := mc.modems[modemIndex]
	if !ok {
		return nil, fmt.Errorf("modem %d not found", modemIndex)
	}

	return &ModemStatus{
		ModemInfo:     m.info,
		State:         m.state,
		Operator:      m.operator,
		SignalQuality: m.signal,
		AccessTech:    "lte",
		IP:            m.ip,
		Interface:     m.info.DataPort,
		ConnectedAt:   m.connected,
		RotationCount: m.rotations,
	}, nil
}

func (mc *mockController) Detail(_ context.Context, modemIndex int) (*ModemDetail, error) {
	mc.mu.RLock()
	m, ok := mc.modems[modemIndex]
	mc.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("modem %d not found", modemIndex)
	}

	return &ModemDetail{
		ModemStatus: ModemStatus{
			ModemInfo:     m.info,
			State:         m.state,
			Operator:      m.operator,
			SignalQuality: m.signal,
			AccessTech:    "lte",
			IP:            m.ip,
			Interface:     m.info.DataPort,
			ConnectedAt:   m.connected,
			RotationCount: m.rotations,
		},
		IMSI:  fmt.Sprintf("28601000000000%d", modemIndex),
		ICCID: fmt.Sprintf("8990100000000000%d", modemIndex),
		APN:   "internet",
		Band:  "B3 (1800 MHz)",
	}, nil
}

func (mc *mockController) Connect(_ context.Context, modemIndex int, _ string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	m, ok := mc.modems[modemIndex]
	if !ok {
		return fmt.Errorf("modem %d not found", modemIndex)
	}

	m.state = StateConnected
	m.ip = randomIPForModem(modemIndex)
	m.connected = time.Now()
	return nil
}

func (mc *mockController) Disconnect(_ context.Context, modemIndex int) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	m, ok := mc.modems[modemIndex]
	if !ok {
		return fmt.Errorf("modem %d not found", modemIndex)
	}

	m.state = StateDisconnected
	m.ip = ""
	return nil
}

func (mc *mockController) SendAT(_ context.Context, serialPort string, command string) (string, error) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	modemIndex := mc.findModemByPort(serialPort)

	switch command {
	case "AT+CGATT=0":
		if m, ok := mc.modems[modemIndex]; ok {
			m.state = StateDisconnected
			m.ip = ""
		}
		return "OK", nil
	case "AT+CGATT=1":
		if m, ok := mc.modems[modemIndex]; ok {
			time.Sleep(500 * time.Millisecond)
			m.state = StateConnected
			m.ip = randomIPForModem(modemIndex)
			m.rotations++
			m.signal = 60 + rand.Intn(35)
		}
		return "OK", nil
	case "AT+CSQ":
		rssi := 10 + rand.Intn(20)
		return fmt.Sprintf("+CSQ: %d,99\r\nOK", rssi), nil
	case "AT+COPS?":
		return "+COPS: 0,0,\"Mock Operator\",7\r\nOK", nil
	default:
		return "OK", nil
	}
}

func (mc *mockController) findModemByPort(serialPort string) int {
	for idx, m := range mc.modems {
		if m.info.SerialPort == serialPort {
			return idx
		}
	}
	return 0
}

var modemIPBlocks = []struct {
	first, second int
}{
	{31, 223},  // Modem 0: 31.223.x.x
	{176, 44},  // Modem 1: 176.44.x.x
	{85, 159},  // Modem 2: 85.159.x.x
	{95, 70},   // Modem 3: 95.70.x.x
}

func randomIPForModem(modemIndex int) string {
	block := modemIPBlocks[modemIndex%len(modemIPBlocks)]
	return fmt.Sprintf("%d.%d.%d.%d",
		block.first,
		block.second,
		rand.Intn(255),
		rand.Intn(254)+1,
	)
}
