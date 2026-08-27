package storage

import (
	"context"
	"fmt"
	"strings"
)

func New(ctx context.Context, driver, localPath, bucket, prefix, region string) (FileStorage, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "", "local":
		return NewLocalFileStorage(localPath)
	case "s3":
		return NewS3FileStorage(ctx, bucket, prefix, region)
	default:
		return nil, fmt.Errorf("STORAGE_DRIVER inválido: %s (use local ou s3)", driver)
	}
}
