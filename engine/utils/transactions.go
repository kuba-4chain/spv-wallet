package utils

import (
	"strings"
)

const efMarker = "0000000000ef"

func IsEf(txHex string) bool {
	return len(txHex) > 20 && strings.EqualFold(txHex[8:20], efMarker)
}
