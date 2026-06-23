//go:build smoke

// Проверка совместимости aws-sdk-go-v2 с Supabase S3 endpoint.
// Запуск: go test -tags smoke -v ./storage/
package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/joho/godotenv"
)

func TestStorageSmoke(t *testing.T) {
	if err := godotenv.Load("../.env"); err != nil {
		t.Fatalf("load .env: %v", err)
	}

	endpoint := os.Getenv("S3_ENDPOINT") // полный URL с /storage/v1/s3
	bucket := os.Getenv("S3_BUCKET")
	region := os.Getenv("S3_REGION")
	access := os.Getenv("S3_ACCESS_KEY")
	secret := os.Getenv("S3_SECRET_ACCESS_KEY")

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(access, secret, "")),
	)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}

	cli := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	key := "smoke/" + time.Now().Format("20060102-150405") + ".txt"
	body := []byte("ponytail smoke test")

	_, err = cli.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("text/plain"),
	})
	if err != nil {
		t.Fatalf("PutObject FAILED: %v", err)
	}
	t.Logf("PutObject OK: %s/%s", bucket, key)

	ps := s3.NewPresignClient(cli)
	req, err := ps.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(5*time.Minute))
	if err != nil {
		t.Fatalf("Presign FAILED: %v", err)
	}
	t.Logf("presigned URL: %s", req.URL)

	resp, err := http.Get(req.URL)
	if err != nil {
		t.Fatalf("GET presigned: %v", err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 || !strings.Contains(string(got), "ponytail") {
		t.Fatalf("presigned download mismatch: status=%d body=%q", resp.StatusCode, string(got))
	}
	t.Logf("presigned download OK: %q", string(got))

	if _, err := cli.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}); err != nil {
		t.Logf("cleanup DeleteObject warn: %v", err)
	}
}
