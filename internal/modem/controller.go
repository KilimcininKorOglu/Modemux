package modem

import "context"

type Controller interface {
	Detect(ctx context.Context) ([]ModemInfo, error)
	Status(ctx context.Context, modemIndex int) (*ModemStatus, error)
	Detail(ctx context.Context, modemIndex int) (*ModemDetail, error)
	Connect(ctx context.Context, modemIndex int, apn string) error
	Disconnect(ctx context.Context, modemIndex int) error
	SendAT(ctx context.Context, serialPort string, command string) (string, error)
}
