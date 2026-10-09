package main

import (
	"bytes"
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// ImageStore holds uploaded images. Keys look like "avatars/<random>.jpg".
type ImageStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	// URL returns an address the browser can load the image from.
	URL(ctx context.Context, key string) (string, error)
}

type s3Images struct {
	client     *s3.Client
	presign    *s3.PresignClient
	bucket     string
	publicBase string
}

// newS3Images builds an S3-backed image store. Credentials and region come
// from the standard AWS chain (AWS_REGION, AWS_ACCESS_KEY_ID /
// AWS_SECRET_ACCESS_KEY, ~/.aws, or an attached IAM role).
//
// endpoint is optional and points at an S3-compatible service such as MinIO.
// publicBase is optional (e.g. a CloudFront URL); without it the bucket can
// stay private and images are served through short-lived presigned URLs.
func newS3Images(ctx context.Context, bucket, endpoint, publicBase string) (*s3Images, error) {
	var opts []func(*config.LoadOptions) error
	if endpoint != "" {
		// S3-compatible services often reject the newer default checksums.
		opts = append(opts,
			config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
			config.WithResponseChecksumValidation(aws.ResponseChecksumValidationWhenRequired))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})
	return &s3Images{
		client:     client,
		presign:    s3.NewPresignClient(client),
		bucket:     bucket,
		publicBase: strings.TrimRight(publicBase, "/"),
	}, nil
}

func (s *s3Images) Put(ctx context.Context, key, contentType string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   aws.String(contentType),
		// Keys are random and never overwritten, so browsers can cache forever.
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	return err
}

func (s *s3Images) URL(ctx context.Context, key string) (string, error) {
	if s.publicBase != "" {
		return s.publicBase + "/" + (&url.URL{Path: key}).EscapedPath(), nil
	}
	request, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Hour))
	if err != nil {
		return "", err
	}
	return request.URL, nil
}
