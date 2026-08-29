package gateway

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Run with DIPOLE_MINIO_PROXY_E2E=1 and an isolated MinIO endpoint. The test
// is opt-in because it creates and removes one object in the configured bucket.
func TestPresignedUploadProxyMinIOEndToEnd(t *testing.T) {
	if os.Getenv("DIPOLE_MINIO_PROXY_E2E") != "1" {
		t.Skip("set DIPOLE_MINIO_PROXY_E2E=1 to run the MinIO integration test")
	}
	endpoint := strings.TrimSpace(os.Getenv("DIPOLE_MINIO_E2E_ENDPOINT"))
	accessKey := os.Getenv("DIPOLE_MINIO_E2E_ACCESS_KEY")
	secretKey := os.Getenv("DIPOLE_MINIO_E2E_SECRET_KEY")
	if endpoint == "" || accessKey == "" || secretKey == "" {
		t.Fatal("DIPOLE_MINIO_E2E_ENDPOINT, DIPOLE_MINIO_E2E_ACCESS_KEY and DIPOLE_MINIO_E2E_SECRET_KEY are required")
	}
	if os.Getenv("DIPOLE_MINIO_E2E_SECURE") == "1" {
		t.Skip("the opt-in smoke currently targets an HTTP MinIO endpoint")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: false})
	if err != nil {
		t.Fatal(err)
	}
	bucket := strings.TrimSpace(os.Getenv("DIPOLE_MINIO_E2E_BUCKET"))
	if bucket == "" {
		bucket = "dipole-files"
	}
	exists, err := client.BucketExists(ctx, bucket)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatalf("bucket %q does not exist", bucket)
	}

	objectKey := "message-files/presigned-proxy-e2e-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".bin"
	core := minio.Core{Client: client}
	uploadID, err := core.NewMultipartUpload(ctx, bucket, objectKey, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = core.AbortMultipartUpload(context.Background(), bucket, objectKey, uploadID)
		_ = client.RemoveObject(context.Background(), bucket, objectKey, minio.RemoveObjectOptions{})
	}()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	signedHost := listener.Addr().String()
	proxy, err := NewPresignedUploadProxy("http://"+endpoint, signedHost, 1024)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/"+bucket+"/", proxy)
	server := httptest.NewUnstartedServer(mux)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	defer server.Close()

	signer, err := minio.New(signedHost, &minio.Options{Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: false})
	if err != nil {
		t.Fatal(err)
	}
	signedURL, err := signer.Presign(ctx, http.MethodPut, bucket, objectKey, 5*time.Minute, url.Values{
		"partNumber": []string{"1"},
		"uploadId":   []string{uploadID},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte("dipole-presigned-proxy-e2e")
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, signedURL.String(), strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(response.Body)
		t.Fatalf("presigned upload failed: status=%d body=%s", response.StatusCode, responseBody)
	}
	etag := strings.Trim(response.Header.Get("ETag"), "\"")
	if etag == "" {
		t.Fatal("MinIO did not return ETag")
	}
	if _, err := core.CompleteMultipartUpload(ctx, bucket, objectKey, uploadID, []minio.CompletePart{{PartNumber: 1, ETag: etag}}, minio.PutObjectOptions{ContentType: "application/octet-stream"}); err != nil {
		t.Fatal(err)
	}

	object, err := client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	result, err := io.ReadAll(object)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(body) {
		t.Fatalf("unexpected object body: %q", result)
	}
}
