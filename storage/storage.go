// Package storage — тонкая обёртка над S3-совместимым хранилищем (Supabase Storage).
// Один конкретный клиент, без интерфейсов: протокол S3 и есть слой портативности.
package storage

import (
	"context"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"tutorgo/config"
)

type Client struct {
	s3      *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// New строит S3-клиент из конфига. Endpoint — полный URL с путём
// (напр. https://<ref>.storage.supabase.co/storage/v1/s3); path-style обязателен.
func New(ctx context.Context, cfg config.Config) (*Client, error) {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(cfg.S3Region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.S3AccessKey, cfg.S3SecretKey, "")),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.S3Endpoint)
		o.UsePathStyle = true
		// Без этого SDK по умолчанию считает CRC32 и шлёт тело в кодировке aws-chunked
		// одним чанком целиком. Supabase режет чанк на 8 МБ → 413 EntityTooLarge на
		// файлах крупнее. WhenRequired отключает чексумму там, где её не требует API,
		// и тело уходит обычным сырым PUT.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	})
	return &Client{
		s3:      client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.S3Bucket,
	}, nil
}

// Put заливает объект целиком.
func (c *Client) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	_, err := c.s3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	return err
}

// PresignGet возвращает временную подписанную ссылку на объект.
func (c *Client) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	req, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Get скачивает объект целиком. Вызывающий закрывает reader.
func (c *Client) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := c.s3.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

// Remove удаляет объект (используется для отката, если запись в БД не удалась).
func (c *Client) Remove(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(key),
	})
	return err
}
