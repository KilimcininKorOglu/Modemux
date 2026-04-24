package modem

import "time"

type State int

const (
	StateDisconnected State = iota
	StateConnecting
	StateConnected
	StateRotating
	StateFailed
)

func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnecting:
		return "connecting"
	case StateConnected:
		return "connected"
	case StateRotating:
		return "rotating"
	case StateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type ModemInfo struct {
	Index        int    `json:"index"`
	Path         string `json:"path"`
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer"`
	SerialPort   string `json:"serialPort"`
	DataPort     string `json:"dataPort"`
	IMEI         string `json:"imei"`
}

type ModemStatus struct {
	ModemInfo
	State         State     `json:"state"`
	Operator      string    `json:"operator"`
	SignalQuality int       `json:"signalQuality"`
	AccessTech    string    `json:"accessTech"`
	IP            string    `json:"ip"`
	Interface     string    `json:"interface"`
	ConnectedAt   time.Time `json:"connectedAt"`
	LastRotation  time.Time `json:"lastRotation"`
	RotationCount int       `json:"rotationCount"`
}

type ModemDetail struct {
	ModemStatus
	IMSI        string `json:"imsi"`
	ICCID       string `json:"iccid"`
	APN         string `json:"apn"`
	Band        string `json:"band"`
	HTTPPort    int    `json:"httpPort"`
	SOCKS5Port  int    `json:"socks5Port"`
}
