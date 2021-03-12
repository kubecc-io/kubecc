package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cobalt77/kubecc/pkg/apps/cachesrv/metrics"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/valyala/bytebufferpool"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
)

var S3StorageError = errors.New("S3 Storage Error")
var ConfigurationError = errors.New("Configuration Error")

type s3StorageProvider struct {
	ctx              context.Context
	lg               *zap.SugaredLogger
	client           *minio.Client
	cfg              config.RemoteStorageSpec
	bucket           string
	cacheHitsTotal   *atomic.Int64
	cacheMissesTotal *atomic.Int64
}

func NewS3StorageProvider(
	ctx context.Context,
	cfg config.RemoteStorageSpec,
) StorageProvider {
	if cfg.Bucket == "" {
		cfg.Bucket = "kubecc"
	}
	sp := &s3StorageProvider{
		ctx:              ctx,
		lg:               meta.Log(ctx),
		cfg:              cfg,
		bucket:           cfg.Bucket,
		cacheHitsTotal:   atomic.NewInt64(0),
		cacheMissesTotal: atomic.NewInt64(0),
	}
	return sp
}

func (*s3StorageProvider) Location() types.StorageLocation {
	return types.S3
}

func (sp *s3StorageProvider) createBucketIfNotExists() error {
	switch exists, err := sp.client.BucketExists(sp.ctx, sp.bucket); {
	case err != nil:
		return fmt.Errorf("%w: %s", S3StorageError, err.Error())
	case !exists:
		sp.lg.Info("Performing first time setup")
		err := sp.client.MakeBucket(sp.ctx, sp.bucket, minio.MakeBucketOptions{
			Region:        sp.cfg.Region,
			ObjectLocking: false,
		})
		if err != nil {
			return fmt.Errorf("%w: Could not create bucket: %s",
				S3StorageError, err.Error())
		}
		lc, err := sp.client.GetBucketLifecycle(sp.ctx, sp.bucket)
		if err != nil {
			return err
		}
		lc.Rules = append(lc.Rules,
			lifecycle.Rule{
				Expiration: lifecycle.Expiration{
					Days: lifecycle.ExpirationDays(sp.cfg.ExpirationDays),
				},
			},
		)
		err = sp.client.SetBucketLifecycle(sp.ctx, sp.bucket, lc)
		if err != nil {
			return fmt.Errorf("%w: Could not configure bucket: %s",
				S3StorageError, err.Error())
		}
		sp.lg.Info("Setup complete")
	default:
		sp.lg.Info("Existing bucket found")
	}
	return nil
}

func (sp *s3StorageProvider) Configure() (err error) {
	sp.client, err = minio.New(sp.cfg.Endpoint, &minio.Options{
		Secure:       sp.cfg.TLS,
		Creds:        credentials.NewStaticV4(sp.cfg.AccessKey, sp.cfg.SecretKey, ""),
		Region:       sp.cfg.Region,
		BucketLookup: minio.BucketLookupAuto,
		Transport:    http.DefaultTransport,
	})
	if err != nil {
		sp.lg.With(
			zap.Error(err),
			zap.String("endpoint", sp.cfg.Endpoint),
		).Info("Error configuring S3 storage provider")
		return
	}
	go func() {
		backoff := wait.NewExponentialBackoffManager(
			2*time.Second,  // Initial
			16*time.Second, // Max
			30*time.Second, // Reset (not used here)
			2.0,            // Factor
			0.1,            // Jitter
			clock.RealClock{},
		)
		for {
			err = sp.createBucketIfNotExists()
			if err != nil {
				sp.lg.With(
					zap.Error(err),
				).Error("Error querying S3 storage")
				<-backoff.Backoff().C()
				continue
			}
			sp.lg.With(
				zap.String("endpoint", sp.client.EndpointURL().String()),
			).Info("S3 storage provider configured")
			return
		}
	}()
	return
}

func (sp *s3StorageProvider) Put(
	ctx context.Context,
	key *types.CacheKey,
	object *types.CacheObject,
) error {
	if object.Metadata == nil {
		object.Metadata = &types.CacheObjectMeta{}
	}
	_, err := sp.client.PutObject(
		sp.ctx,
		sp.bucket,
		key.GetHash(),
		bytes.NewReader(object.Data),
		int64(len(object.Data)),
		minio.PutObjectOptions{
			UserMetadata: map[string]string{
				"timestamp": strconv.FormatInt(time.Now().UnixNano(), 10),
				"score":     "1",
			},
			UserTags:    object.Metadata.GetTags(),
			ContentType: "application/octet-stream",
		},
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func (sp *s3StorageProvider) Get(
	ctx context.Context,
	key *types.CacheKey,
) (*types.CacheObject, error) {
	// Check if the object exists
	hash := key.GetHash()
	info, err := sp.client.StatObject(
		ctx, sp.bucket, hash, minio.GetObjectOptions{})
	if err != nil {
		// Not found
		sp.cacheMissesTotal.Inc()
		return nil, status.Error(codes.NotFound,
			fmt.Errorf("Object not found: %w", err).Error())
	}
	objectBuf := bytebufferpool.Get()
	defer bytebufferpool.Put(objectBuf)
	done := make(chan error)

	go func() {
		// Start streaming object data from s3
		obj, err := sp.client.GetObject(
			sp.ctx,
			sp.bucket,
			hash,
			minio.GetObjectOptions{},
		)
		if err != nil {
			// Something went wrong, but the object exists
			done <- status.Error(codes.NotFound,
				fmt.Errorf("Error retrieving object: %w", err).Error())
		}
		_, err = objectBuf.ReadFrom(obj)
		if err != nil {
			done <- status.Error(codes.Internal, err.Error())
		}
		done <- nil
		close(done)
	}()

	// Increment the score by 1
	score := int64(1)
	metadata := info.UserMetadata
	if value, ok := metadata["score"]; ok {
		s, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			score = s
		}
	}
	score++
	metadata["score"] = strconv.FormatInt(score, 10)

	// Copy object to itself and replace the metadata
	go func() {
		_, err := sp.client.CopyObject(sp.ctx,
			minio.CopyDestOptions{
				Bucket:          sp.bucket,
				Object:          hash,
				UserMetadata:    metadata,
				ReplaceMetadata: true,
			},
			minio.CopySrcOptions{
				Bucket: sp.bucket,
				Object: hash,
			})
		if err != nil {
			sp.lg.With(zap.Error(err)).Error("Failed to update object")
		}
	}()
	sp.cacheHitsTotal.Inc()

	// Wait for read to complete, or context canceled
	select {
	case <-done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return &types.CacheObject{
		Data: objectBuf.Bytes(),
		Metadata: &types.CacheObjectMeta{
			Tags:           info.UserTags,
			ExpirationDate: info.Expiration.UnixNano(),
			ManagedFields: &types.CacheObjectManaged{
				Size:      info.Size,
				Timestamp: time.Now().UnixNano(),
				Score:     score,
				Location:  types.S3,
			},
		},
	}, nil
}

func (sp *s3StorageProvider) Query(
	ctx context.Context,
	keys []*types.CacheKey,
) ([]*types.CacheObjectMeta, error) {
	results := make([]*types.CacheObjectMeta, len(keys))
	for i, key := range keys {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		info, err := sp.client.StatObject(
			ctx, sp.bucket, key.GetHash(), minio.GetObjectOptions{})
		if err != nil {
			continue
		}
		timestamp, err := strconv.ParseInt(info.UserMetadata["timestamp"], 10, 64)
		if err != nil {
			sp.lg.Debug(err)
			continue
		}
		score, err := strconv.ParseInt(info.UserMetadata["score"], 10, 64)
		if err != nil {
			sp.lg.Debug(err)
			continue
		}
		results[i] = &types.CacheObjectMeta{
			Tags:           info.UserTags,
			ExpirationDate: info.Expiration.UnixNano(),
			ManagedFields: &types.CacheObjectManaged{
				Timestamp: timestamp,
				Score:     score,
				Size:      info.Size,
				Location:  types.S3,
			},
		}
	}
	return results, nil
}

func (sp *s3StorageProvider) UsageInfo() *metrics.UsageInfo {
	info := &metrics.UsageInfo{
		ObjectCount: 0,
		TotalSize:   0,
	}
	for object := range sp.client.ListObjects(sp.ctx, sp.bucket, minio.ListObjectsOptions{}) {
		info.ObjectCount++
		info.TotalSize += object.Size
	}
	return info
}

func (sp *s3StorageProvider) CacheHits() *metrics.CacheHits {
	hitTotal := sp.cacheHitsTotal.Load()
	missTotal := sp.cacheMissesTotal.Load()
	var percent float64
	if hitTotal+missTotal == 0 {
		percent = 0
	} else {
		percent = float64(hitTotal) / float64(hitTotal+missTotal)
	}
	return &metrics.CacheHits{
		CacheHitsTotal:   hitTotal,
		CacheMissesTotal: missTotal,
		CacheHitPercent:  percent,
	}
}
