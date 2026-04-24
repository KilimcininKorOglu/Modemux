package templates

import (
	"fmt"
	"strconv"
)

func itoa(n int) string {
	return strconv.Itoa(n)
}

func formatDuration(ms int64) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

var AssetVersion = "dev"

func staticURL(path string) string {
	return fmt.Sprintf("%s?v=%s", path, AssetVersion)
}

func signalBars(quality int) int {
	switch {
	case quality >= 80:
		return 5
	case quality >= 60:
		return 4
	case quality >= 40:
		return 3
	case quality >= 20:
		return 2
	case quality > 0:
		return 1
	default:
		return 0
	}
}
