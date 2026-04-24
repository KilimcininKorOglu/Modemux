//go:build !dev

package modem

import "fmt"

func NewMockController(_ int) Controller {
	panic(fmt.Sprintf("mock controller is not available in production builds; rebuild with: go build -tags dev"))
}
