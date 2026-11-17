// Package s3 uploads recordings to S3-compatible object storage.
package s3

import (
	"context"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

type Options struct {
	Endpoint, Region, Bucket, AccessKey, SecretKey string
}

type Uploader struct {
	u      *manager.Uploader
	bucket string
}

func New(ctx context.Context, c Options) (*Uploader, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(c.Region)}
	if c.AccessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")))
	}
	cfg, e := awsconfig.LoadDefaultConfig(ctx, opts...)
	if e != nil {
		return nil, e
	}
	client := awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.UsePathStyle = true
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
	})
	u := manager.NewUploader(client, func(u *manager.Uploader) { u.Concurrency = 2; u.PartSize = 8 * 1024 * 1024 })
	return &Uploader{u, c.Bucket}, nil
}
func (s *Uploader) Put(ctx context.Context, key, file string) error {
	f, e := os.Open(file)
	if e != nil {
		return e
	}
	defer f.Close()
	_, e = s.u.Upload(ctx, &awss3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: f, ContentType: aws.String("video/x-matroska")})
	return e
}
