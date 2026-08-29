package gateway

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

// NewPresignedUploadProxy creates a same-origin data-plane proxy for S3
// UploadPart requests. The S3 signature remains the authorization mechanism;
// the Gateway only forwards a bounded PUT and never calls a business service.
func NewPresignedUploadProxy(target, signedHost string, maxBodyBytes int64) (http.Handler, error) {
	parsed, err := url.Parse(strings.TrimSpace(target))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("presigned upload proxy target is invalid")
	}
	signedHost = strings.TrimSpace(signedHost)
	if signedHost == "" {
		return nil, errors.New("presigned upload proxy signed host is required")
	}
	if maxBodyBytes <= 0 {
		return nil, errors.New("presigned upload proxy body limit must be positive")
	}

	proxy := httputil.NewSingleHostReverseProxy(parsed)
	director := proxy.Director
	proxy.Director = func(request *http.Request) {
		director(request)
		// The URL is signed for the externally advertised host. Keep that host
		// in the upstream request while routing the TCP connection internally.
		request.Host = signedHost
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPut || !hasMultipartSignature(request) {
			writer.WriteHeader(http.StatusForbidden)
			return
		}
		if request.ContentLength > maxBodyBytes {
			writer.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, maxBodyBytes)
		proxy.ServeHTTP(writer, request)
	}), nil
}

func hasMultipartSignature(request *http.Request) bool {
	query := request.URL.Query()
	if query.Get("X-Amz-Signature") == "" || query.Get("X-Amz-Credential") == "" {
		return false
	}
	if query.Get("uploadId") == "" || query.Get("partNumber") == "" {
		return false
	}
	partNumber, err := strconv.Atoi(query.Get("partNumber"))
	return err == nil && partNumber > 0
}
