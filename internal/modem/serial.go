package modem

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
)

type SerialATSender struct {
	baudRate int
	timeout  time.Duration
}

func NewSerialATSender() *SerialATSender {
	return &SerialATSender{
		baudRate: 115200,
		timeout:  5 * time.Second,
	}
}

func (s *SerialATSender) Send(ctx context.Context, portPath string, command string) (string, error) {
	mode := &serial.Mode{
		BaudRate: s.baudRate,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	port, err := serial.Open(portPath, mode)
	if err != nil {
		return "", fmt.Errorf("opening serial port %s: %w", portPath, err)
	}
	defer port.Close()

	if err := port.SetReadTimeout(s.timeout); err != nil {
		return "", fmt.Errorf("setting read timeout: %w", err)
	}

	cmd := command + "\r\n"
	if _, err := port.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("writing AT command: %w", err)
	}

	response, err := s.readResponse(ctx, port)
	if err != nil {
		return "", err
	}

	return response, nil
}

func (s *SerialATSender) readResponse(ctx context.Context, port serial.Port) (string, error) {
	var buf strings.Builder
	tmp := make([]byte, 256)

	deadline := time.After(s.timeout)
	for {
		select {
		case <-ctx.Done():
			return buf.String(), ctx.Err()
		case <-deadline:
			return buf.String(), nil
		default:
		}

		n, err := port.Read(tmp)
		if err != nil {
			return buf.String(), nil
		}
		if n > 0 {
			buf.Write(tmp[:n])
			resp := buf.String()
			if strings.Contains(resp, "OK") || strings.Contains(resp, "ERROR") {
				return strings.TrimSpace(resp), nil
			}
		}
	}
}

func (s *SerialATSender) Detach(ctx context.Context, portPath string) error {
	resp, err := s.Send(ctx, portPath, "AT+CGATT=0")
	if err != nil {
		return fmt.Errorf("detach (AT+CGATT=0): %w", err)
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("detach failed: %s", resp)
	}
	return nil
}

func (s *SerialATSender) Attach(ctx context.Context, portPath string) error {
	resp, err := s.Send(ctx, portPath, "AT+CGATT=1")
	if err != nil {
		return fmt.Errorf("attach (AT+CGATT=1): %w", err)
	}
	if strings.Contains(resp, "ERROR") {
		return fmt.Errorf("attach failed: %s", resp)
	}
	return nil
}

func (s *SerialATSender) SignalQuality(ctx context.Context, portPath string) (int, error) {
	resp, err := s.Send(ctx, portPath, "AT+CSQ")
	if err != nil {
		return 0, err
	}

	// Response format: +CSQ: 15,99
	idx := strings.Index(resp, "+CSQ:")
	if idx < 0 {
		return 0, fmt.Errorf("unexpected CSQ response: %s", resp)
	}

	parts := strings.Split(resp[idx+5:], ",")
	if len(parts) < 1 {
		return 0, fmt.Errorf("invalid CSQ format: %s", resp)
	}

	rssi := strings.TrimSpace(parts[0])
	var val int
	if _, err := fmt.Sscanf(rssi, "%d", &val); err != nil {
		return 0, err
	}

	// Convert RSSI (0-31) to percentage (0-100)
	if val == 99 {
		return 0, nil
	}
	return int(float64(val) / 31.0 * 100.0), nil
}

func (s *SerialATSender) CurrentOperator(ctx context.Context, portPath string) (string, error) {
	resp, err := s.Send(ctx, portPath, "AT+COPS?")
	if err != nil {
		return "", err
	}

	// Response format: +COPS: 0,0,"Turkcell",7
	idx := strings.Index(resp, "\"")
	if idx < 0 {
		return "", nil
	}
	end := strings.Index(resp[idx+1:], "\"")
	if end < 0 {
		return "", nil
	}
	return resp[idx+1 : idx+1+end], nil
}
