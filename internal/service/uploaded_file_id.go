package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

func generateUploadedFileUUID() string {
	buf := make([]byte, 10)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Errorf("generate uploaded file uuid: %w", err))
	}

	return "F" + strings.ToUpper(hex.EncodeToString(buf))
}
