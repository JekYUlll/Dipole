package gateway

import "github.com/JekYUlll/Dipole/internal/dto/httpdto"

// These declarations document Gateway-owned endpoints without coupling the
// service package to the Core HTTP handler implementation.
type SearchMessageListResponseEnvelope struct {
	Code int                              `json:"code"`
	Data []*httpdto.SearchMessageResponse `json:"data"`
}

type ErrorEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
