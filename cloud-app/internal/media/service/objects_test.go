package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"io"
	"kicau.cloud/control/internal/media/model"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func exerciseObject(t *testing.T, o *Objects, key string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	data := "fixture-mp4-payload"
	sum := sha256.Sum256([]byte(data))
	hexHash := hex.EncodeToString(sum[:])
	checksum := base64.StdEncoding.EncodeToString(sum[:])
	id, e := o.Begin(ctx, key, hexHash)
	if e != nil {
		t.Fatal("begin", e)
	}
	signed, e := o.Sign(ctx, key, id, model.SignPart{Number: 1, SHA256: checksum})
	if e != nil {
		t.Fatal("sign", e)
	}
	req, e := http.NewRequestWithContext(ctx, "PUT", signed.URL, strings.NewReader(data))
	if e != nil {
		t.Fatal(e)
	}
	for k, v := range signed.Headers {
		req.Header.Set(k, v)
	}
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal("put", e)
	}
	io.Copy(io.Discard, res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("put status", res.StatusCode)
	}
	parts, e := o.Parts(ctx, key, id)
	if e != nil || len(parts) != 1 || parts[0].SHA256 != checksum {
		t.Fatal("parts", parts, e)
	}
	if e = o.Complete(ctx, key, id, parts); e != nil {
		t.Fatal("complete", e)
	}
	ok, e := o.Exists(ctx, key, int64(len(data)), hexHash)
	if e != nil || !ok {
		t.Fatal("head", e)
	}
	download, e := o.Download(ctx, key)
	if e != nil {
		t.Fatal("download", e)
	}
	if name := model.RecordingName(key); name != "" {
		signed, err := url.Parse(download)
		if err != nil || !strings.Contains(signed.Query().Get("response-content-disposition"), name) {
			t.Fatal("missing readable download name")
		}
	}
}
func TestS3SDKMultipartContract(t *testing.T) {
	for _, key := range []string{"fixture.mp4", "fixture.mkv", "Gantangan 03 - Rekaman Terbatas.mkv"} {
		t.Run(key, func(t *testing.T) {
			data := "fixture-mp4-payload"
			sum := sha256.Sum256([]byte(data))
			checksum := base64.StdEncoding.EncodeToString(sum[:])
			hash := hex.EncodeToString(sum[:])
			var got string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/xml")
				switch {
				case r.Method == "POST" && r.URL.Query().Has("uploads"):
					if r.Header.Get("Content-Type") != contentType(key) {
						t.Error("wrong container MIME type")
					}
					fmt.Fprint(w, `<InitiateMultipartUploadResult><Bucket>kicau-test-sdk</Bucket><Key>fixture.mp4</Key><UploadId>upload01</UploadId></InitiateMultipartUploadResult>`)
				case r.Method == "PUT":
					b, _ := io.ReadAll(r.Body)
					got = string(b)
					w.Header().Set("ETag", `"fixture"`)
				case r.Method == "GET" && r.URL.Query().Has("uploadId"):
					fmt.Fprintf(w, `<ListPartsResult><Bucket>kicau-test-sdk</Bucket><Key>fixture.mp4</Key><UploadId>upload01</UploadId><IsTruncated>false</IsTruncated><Part><PartNumber>1</PartNumber><ETag>fixture</ETag><Size>%d</Size><ChecksumSHA256>%s</ChecksumSHA256></Part></ListPartsResult>`, len(data), checksum)
				case r.Method == "POST":
					fmt.Fprint(w, `<CompleteMultipartUploadResult><Location>fixture</Location><Bucket>kicau-test-sdk</Bucket><Key>fixture.mp4</Key><ETag>fixture</ETag></CompleteMultipartUploadResult>`)
				case r.Method == "HEAD":
					w.Header().Set("Content-Type", contentType(key))
					w.Header().Set("Content-Length", fmt.Sprint(len(data)))
					w.Header().Set("X-Amz-Meta-Sha256", hash)
				}
			}))
			defer server.Close()
			o, e := NewObjects(context.Background(), server.URL, "us-east-1", "kicau-test-sdk", "fixture-key", "fixture-secret")
			if e != nil {
				t.Fatal(e)
			}
			exerciseObject(t, o, key)
			if got != data {
				t.Fatal("payload changed")
			}

		})
	}
}

func TestIsolatedS3Multipart(t *testing.T) {
	endpoint, bucket, key, secret := os.Getenv("KICAU_TEST_S3_ENDPOINT"), os.Getenv("KICAU_TEST_S3_BUCKET"), os.Getenv("KICAU_TEST_S3_ACCESS_KEY"), os.Getenv("KICAU_TEST_S3_SECRET_KEY")
	if endpoint == "" || bucket == "" || key == "" || secret == "" {
		t.Skip("Explicit isolated S3/MinIO test environment required")
	}
	if !strings.HasPrefix(bucket, "kicau-test-") {
		t.Fatal("Refusing a non-test bucket")
	}
	o, e := NewObjects(context.Background(), endpoint, "us-east-1", bucket, key, secret)
	if e != nil {
		t.Fatal(e)
	}
	object := fmt.Sprintf("desktop-test/%d.mp4", time.Now().UnixNano())
	defer o.Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(object)})
	exerciseObject(t, o, object)
}
