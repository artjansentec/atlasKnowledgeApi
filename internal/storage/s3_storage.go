package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/google/uuid"
)

type S3FileStorage struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3FileStorage(ctx context.Context, bucket, prefix, region string) (*S3FileStorage, error) {
	var loadOpts []func(*awsconfig.LoadOptions) error
	if region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(region))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("aws config (IAM role da EC2): %w", err)
	}

	region = awsCfg.Region
	if region == "" {
		region = "us-east-1"
	}

	store := &S3FileStorage{
		client: s3.NewFromConfig(awsCfg),
		bucket: bucket,
		prefix: strings.Trim(prefix, "/"),
	}
	if err := store.ensureBucket(ctx, region); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *S3FileStorage) ensureBucket(ctx context.Context, region string) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err == nil {
		return nil
	}
	if !isBucketMissing(err) {
		return fmt.Errorf("s3 bucket %s: %w", s.bucket, err)
	}

	input := &s3.CreateBucketInput{Bucket: aws.String(s.bucket)}
	if region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(region),
		}
	}
	if _, err := s.client.CreateBucket(ctx, input); err != nil {
		var owned *types.BucketAlreadyOwnedByYou
		if errors.As(err, &owned) {
			return nil
		}
		return fmt.Errorf("s3 create bucket %s: %w", s.bucket, err)
	}

	waiter := s3.NewBucketExistsWaiter(s.client)
	if err := waiter.Wait(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}, 2*time.Minute); err != nil {
		return fmt.Errorf("s3 wait bucket %s: %w", s.bucket, err)
	}
	return nil
}

func isBucketMissing(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchBucket", "404":
			return true
		}
	}
	return false
}

func (s *S3FileStorage) objectKey(storageKey string) string {
	base := path.Base(strings.ReplaceAll(storageKey, "\\", "/"))
	if s.prefix == "" {
		return base
	}
	return s.prefix + "/" + base
}

func (s *S3FileStorage) Save(ctx context.Context, originalName string, reader io.Reader) (string, error) {
	key := uuid.NewString() + filepath.Ext(originalName)
	contentType := mime.TypeByExtension(filepath.Ext(originalName))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	uploader := manager.NewUploader(s.client)
	if _, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(s.objectKey(key)),
		Body:        reader,
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", fmt.Errorf("s3 put: %w", err)
	}
	return key, nil
}

func (s *S3FileStorage) Open(ctx context.Context, storageKey string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(storageKey)),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, fmt.Errorf("s3 get: %w", err)
		}
		return nil, fmt.Errorf("s3 get: %w", err)
	}
	return out.Body, nil
}

func (s *S3FileStorage) Delete(ctx context.Context, storageKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(s.objectKey(storageKey)),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil
		}
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}
