package service

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/media/model"
	"path"
	"strings"
	"time"
)

type Objects struct {
	Client *s3.Client
	Bucket string
}

func NewObjects(c context.Context, endpoint, region, bucket, key, secret string) (*Objects, error) {
	cfg, e := awsconfig.LoadDefaultConfig(c, awsconfig.WithRegion(region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(key, secret, "")))
	if e != nil {
		return nil, e
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
	return &Objects{client, bucket}, nil
}
func failure(e error) error {
	if e == nil {
		return nil
	}
	var a smithy.APIError
	if errors.As(e, &a) {
		switch a.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchUpload":
			return apperror.ErrNotFound
		}
	}
	return apperror.ErrUnavailable
}
func (o *Objects) Begin(c context.Context, key, hash string) (string, error) {
	r, e := o.Client.CreateMultipartUpload(c, &s3.CreateMultipartUploadInput{Bucket: aws.String(o.Bucket), Key: aws.String(key), ContentType: aws.String(contentType(key)), Metadata: map[string]string{"sha256": hash}, ChecksumAlgorithm: types.ChecksumAlgorithmSha256})
	if e != nil {
		return "", failure(e)
	}
	return aws.ToString(r.UploadId), nil
}
func (o *Objects) Sign(c context.Context, key, id string, p model.SignPart) (model.Signed, error) {
	b, e := base64.StdEncoding.DecodeString(p.SHA256)
	if e != nil || len(b) != 32 {
		return model.Signed{}, apperror.ErrInvalid
	}
	r, e := s3.NewPresignClient(o.Client).PresignUploadPart(c, &s3.UploadPartInput{Bucket: aws.String(o.Bucket), Key: aws.String(key), UploadId: aws.String(id), PartNumber: aws.Int32(p.Number), ChecksumSHA256: aws.String(p.SHA256)}, s3.WithPresignExpires(15*time.Minute))
	if e != nil {
		return model.Signed{}, failure(e)
	}
	headers := map[string]string{}
	for k, v := range r.SignedHeader {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	headers["X-Amz-Checksum-Sha256"] = p.SHA256
	return model.Signed{URL: r.URL, Headers: headers}, nil
}
func (o *Objects) Parts(c context.Context, key, id string) ([]model.Part, error) {
	out := []model.Part{}
	p := s3.NewListPartsPaginator(o.Client, &s3.ListPartsInput{Bucket: aws.String(o.Bucket), Key: aws.String(key), UploadId: aws.String(id)})
	for p.HasMorePages() {
		r, e := p.NextPage(c)
		if e != nil {
			return nil, failure(e)
		}
		for _, v := range r.Parts {
			out = append(out, model.Part{Number: aws.ToInt32(v.PartNumber), ETag: aws.ToString(v.ETag), SHA256: aws.ToString(v.ChecksumSHA256)})
		}
	}
	return out, nil
}
func (o *Objects) Complete(c context.Context, key, id string, parts []model.Part) error {
	p := []types.CompletedPart{}
	for _, v := range parts {
		p = append(p, types.CompletedPart{PartNumber: aws.Int32(v.Number), ETag: aws.String(v.ETag), ChecksumSHA256: aws.String(v.SHA256)})
	}
	_, e := o.Client.CompleteMultipartUpload(c, &s3.CompleteMultipartUploadInput{Bucket: aws.String(o.Bucket), Key: aws.String(key), UploadId: aws.String(id), MultipartUpload: &types.CompletedMultipartUpload{Parts: p}})
	return failure(e)
}
func (o *Objects) Exists(c context.Context, key string, size int64, hash string) (bool, error) {
	r, e := o.Client.HeadObject(c, &s3.HeadObjectInput{Bucket: aws.String(o.Bucket), Key: aws.String(key)})
	if errors.Is(failure(e), apperror.ErrNotFound) {
		return false, nil
	}
	if e != nil {
		return false, failure(e)
	}
	if aws.ToInt64(r.ContentLength) != size || r.Metadata["sha256"] != hash || aws.ToString(r.ContentType) != contentType(key) {
		return false, apperror.ErrConflict
	}
	return true, nil
}
func (o *Objects) Download(c context.Context, key string) (string, error) {
	r, e := s3.NewPresignClient(o.Client).PresignGetObject(c, &s3.GetObjectInput{Bucket: aws.String(o.Bucket), Key: aws.String(key), ResponseContentDisposition: aws.String("attachment; filename=recording" + path.Ext(key))}, s3.WithPresignExpires(time.Minute))
	if e != nil {
		return "", failure(e)
	}
	return r.URL, nil
}

func contentType(key string) string {
	if strings.EqualFold(path.Ext(key), ".mkv") {
		return "video/x-matroska"
	}
	return "video/mp4"
}
