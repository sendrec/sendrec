package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	maxBytes  int64
}

type Config struct {
	Endpoint       string
	PublicEndpoint string // Used for presigned URLs; falls back to Endpoint if empty
	Bucket         string
	AccessKey      string
	SecretKey      string
	Region         string
	MaxUploadBytes int64
}

func New(ctx context.Context, cfg Config) (*Storage, error) {
	if cfg.Region == "" {
		cfg.Region = "eu-central-1"
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})

	presignEndpoint := cfg.Endpoint
	if cfg.PublicEndpoint != "" {
		presignEndpoint = cfg.PublicEndpoint
	}
	presignClient := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(presignEndpoint)
		o.UsePathStyle = true
	})
	presigner := s3.NewPresignClient(presignClient)

	return &Storage{
		client:    client,
		presigner: presigner,
		bucket:    cfg.Bucket,
		maxBytes:  cfg.MaxUploadBytes,
	}, nil
}

func (s *Storage) GenerateUploadURL(ctx context.Context, key string, contentType string, contentLength int64, expiry time.Duration) (string, error) {
	if s == nil {
		return "", fmt.Errorf("storage not initialized")
	}
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}
	if s.uploadLimit() > 0 && contentLength > s.uploadLimit() {
		return "", fmt.Errorf("file too large: %d > %d", contentLength, s.uploadLimit())
	}
	if contentLength > 0 {
		input.ContentLength = aws.Int64(contentLength)
	}
	req, err := s.presigner.PresignPutObject(ctx, input, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign upload: %w", err)
	}

	return req.URL, nil
}

func (s *Storage) GenerateDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign download: %w", err)
	}

	return req.URL, nil
}

func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '"' || r == '\\' || r < 0x20 {
			b.WriteRune('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Storage) GenerateDownloadURLWithDisposition(ctx context.Context, key string, filename string, expiry time.Duration) (string, error) {
	disposition := fmt.Sprintf(`attachment; filename="%s"`, sanitizeFilename(filename))
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("presign download: %w", err)
	}

	return req.URL, nil
}

func (s *Storage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

func (s *Storage) uploadLimit() int64 {
	return s.maxBytes
}

func (s *Storage) HeadObject(ctx context.Context, key string) (int64, string, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return 0, "", fmt.Errorf("head object: %w", err)
	}
	size := int64(0)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	ct := ""
	if out.ContentType != nil {
		ct = *out.ContentType
	}
	return size, ct, nil
}

func (s *Storage) SetCORS(ctx context.Context, allowedOrigins []string) error {
	_, err := s.client.PutBucketCors(ctx, &s3.PutBucketCorsInput{
		Bucket: aws.String(s.bucket),
		CORSConfiguration: &types.CORSConfiguration{
			CORSRules: []types.CORSRule{
				{
					AllowedOrigins: allowedOrigins,
					AllowedMethods: []string{"GET", "PUT"},
					AllowedHeaders: []string{"*"},
					MaxAgeSeconds:  aws.Int32(3600),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("set bucket CORS: %w", err)
	}
	return nil
}

func (s *Storage) EnsureBucket(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return nil
	}

	_, err = s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("create bucket: %w", err)
	}

	return nil
}

func (s *Storage) DownloadToFile(ctx context.Context, key string, destPath string) error {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get object %s: %w", key, err)
	}
	defer func() { _ = out.Body.Close() }()

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}

	if _, err := io.Copy(f, out.Body); err != nil {
		_ = f.Close()
		return fmt.Errorf("write file %s: %w", destPath, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("sync file %s: %w", destPath, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close file %s: %w", destPath, err)
	}
	return nil
}

func (s *Storage) UploadFile(ctx context.Context, key string, filePath string, contentType string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("upload file %s: %w", key, err)
	}
	return nil
}
