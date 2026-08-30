package gateway

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultPresignedUploadProxyTimeout = 30 * time.Second

// NewPresignedUploadProxy creates a same-origin data-plane proxy for S3
// UploadPart requests. The S3 signature remains the authorization mechanism;
// the Gateway only forwards a bounded PUT and never calls a business service.
func NewPresignedUploadProxy(target, signedHost string, maxBodyBytes int64) (http.Handler, error) {
	return NewPresignedUploadProxyWithTimeout(target, signedHost, maxBodyBytes, defaultPresignedUploadProxyTimeout)
}

// NewPresignedUploadProxyWithTimeout creates a bounded same-origin S3 part
// proxy. The timeout limits waiting for the upstream response and prevents a
// stalled object store from pinning a Gateway request indefinitely.
func NewPresignedUploadProxyWithTimeout(target, signedHost string, maxBodyBytes int64, upstreamTimeout time.Duration) (http.Handler, error) {
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
	if upstreamTimeout <= 0 {
		return nil, errors.New("presigned upload proxy upstream timeout must be positive")
	}

	proxy := httputil.NewSingleHostReverseProxy(parsed)
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, errors.New("presigned upload proxy default transport is not cloneable")
	}
	proxy.Transport = transport.Clone()
	proxy.Transport.(*http.Transport).ResponseHeaderTimeout = upstreamTimeout
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
