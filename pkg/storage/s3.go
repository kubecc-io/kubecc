package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/cobalt77/kubecc/pkg/apps/cachesrv/metrics"
	"github.com/cobalt77/kubecc/pkg/config"
	"github.com/cobalt77/kubecc/pkg/meta"
	"github.com/cobalt77/kubecc/pkg/types"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/api/resource"
)

var S3StorageError = errors.New("S3 Storage Error")
var ConfigurationError = errors.New("Configuration Error")

type s3StorageProvider struct {
	ctx               context.Context
	lg                *zap.SugaredLogger
	client            *minio.Client
	cfg               config.RemoteStorageSpec
	bucket            string
	knownObjects      map[string]*types.CacheObjectMeta
	knownObjectsMutex *sync.RWMutex
	totalSize         *atomic.Int64
	storageLimitBytes int64
	cacheHitsTotal    *atomic.Int64
	cacheMissesTotal  *atomic.Int64
}

func NewS3StorageProvider(
	ctx context.Context,
	cfg config.RemoteStorageSpec,
) StorageProvider {
	if cfg.Bucket == "" {
		cfg.Bucket = "kubecc"
	}
	sp := &s3StorageProvider{
		ctx:               ctx,
		lg:                meta.Log(ctx).Named("s3"),
		cfg:               cfg,
		bucket:            cfg.Bucket,
		knownObjects:      make(map[string]*types.CacheObjectMeta),
		knownObjectsMutex: &sync.RWMutex{},
		totalSize:         atomic.NewInt64(0),
		cacheHitsTotal:    atomic.NewInt64(0),
		cacheMissesTotal:  atomic.NewInt64(0),
	}
	q, err := resource.ParseQuantity(sp.cfg.Limits.Disk)
	if err != nil {
		sp.lg.Fatal("%w: %s", ConfigurationError, err)
	}
	sp.storageLimitBytes = q.Value()
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
		err := sp.client.MakeBucket(sp.ctx, sp.bucket, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("%w: Could not create bucket: %s",
				S3StorageError, err.Error())
		}
		sp.lg.Info("Setup complete")
	default:
		sp.lg.Info("Existing bucket found")
	}
	return nil
}

func (sp *s3StorageProvider) Configure() error {
	sp.knownObjectsMutex.Lock()
	defer sp.knownObjectsMutex.Unlock()

	client, err := minio.New(sp.cfg.Endpoint, &minio.Options{
		Secure: sp.cfg.TLS,
		Creds:  credentials.NewStaticV4(sp.cfg.AccessKey, sp.cfg.SecretKey, ""),
	})
	if err != nil {
		return err
	}
	sp.lg.With(
		zap.String("endpoint", client.EndpointURL().String()),
	).Info("Connected to S3 storage provider")
	sp.client = client
	err = sp.createBucketIfNotExists()
	if err != nil {
		return err
	}
	objects := sp.client.ListObjects(sp.ctx, sp.bucket, minio.ListObjectsOptions{
		WithMetadata: true,
		WithVersions: false,
	})
	for object := range objects {
		seconds, err := strconv.ParseInt(object.UserTags["cpuSecondsUsed"], 10, 64)
		if err != nil {
			sp.lg.With(zap.Error(err)).Error("Invalid tag value")
			continue
		}
		score, err := strconv.ParseInt(object.UserTags["score"], 10, 64)
		if err != nil {
			sp.lg.With(zap.Error(err)).Error("Invalid tag value")
			continue
		}
		timestamp, err := strconv.ParseInt(object.UserTags["timestamp"], 10, 64)
		if err != nil {
			sp.lg.With(zap.Error(err)).Error("Invalid tag value")
			continue
		}
		sp.knownObjects[object.Key] = &types.CacheObjectMeta{
			CpuSecondsUsed: seconds,
			ExpirationTime: object.Expiration.Unix(),
			ManagedFields: &types.CacheObjectManaged{
				Size:      object.Size,
				Timestamp: timestamp,
				Score:     score,
				Location:  types.S3,
			},
		}
		sp.totalSize.Add(object.Size)
	}
	sp.lg.Infof("Loaded metadata for %d objects from S3 storage",
		len(sp.knownObjects))
	return nil
}

func (sp *s3StorageProvider) Put(
	ctx context.Context,
	key *types.CacheKey,
	object *types.CacheObject,
) error {
	sp.knownObjectsMutex.Lock()
	defer sp.knownObjectsMutex.Unlock()
	if _, ok := sp.knownObjects[key.GetHash()]; ok {
		return status.Error(codes.AlreadyExists, "Object already exists")
	}
	meta := object.Metadata
	meta.ManagedFields.Timestamp = time.Now().Unix()
	meta.ManagedFields.Score = 1
	meta.ManagedFields.Location = types.S3
	info, err := sp.client.PutObject(
		sp.ctx,
		sp.bucket,
		key.GetHash(),
		bytes.NewReader(object.Data),
		int64(len(object.Data)),
		minio.PutObjectOptions{
			UserMetadata: map[string]string{
				"timestamp":      strconv.FormatInt(meta.ManagedFields.Timestamp, 10),
				"cpuSecondsUsed": strconv.FormatInt(meta.CpuSecondsUsed, 10),
				"score":          strconv.FormatInt(meta.ManagedFields.Score, 10),
			},
			UserTags:        key.GetTags(),
			ContentType:     "application/octet-stream",
			RetainUntilDate: time.Unix(object.Metadata.ExpirationTime, 0),
		},
	)
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	meta.ManagedFields.Size = info.Size
	sp.knownObjects[key.Hash] = meta
	sp.totalSize.Add(info.Size)
	return nil
}

func (sp *s3StorageProvider) Get(
	ctx context.Context,
	key *types.CacheKey,
) (*types.CacheObject, error) {
	sp.knownObjectsMutex.RLock()
	hash := key.GetHash()
	info, ok := sp.knownObjects[hash]
	if !ok {
		sp.knownObjectsMutex.RUnlock()
		sp.cacheMissesTotal.Inc()
		return nil, status.Error(codes.NotFound, "Object not found")
	}
	sp.knownObjectsMutex.RUnlock()

	obj, err := sp.client.GetObject(
		sp.ctx,
		sp.bucket,
		hash,
		minio.GetObjectOptions{},
	)
	if err != nil {
		sp.cacheMissesTotal.Inc()

		// todo
		// Object expired, remove it from our cache
		sp.knownObjectsMutex.Lock()
		defer sp.knownObjectsMutex.Unlock()
		meta := sp.knownObjects[hash]
		delete(sp.knownObjects, hash)
		sp.totalSize.Sub(meta.ManagedFields.Size)

		return nil, status.Error(codes.NotFound,
			fmt.Errorf("Object expired: %w", err).Error())
	}
	sp.cacheHitsTotal.Inc()
	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, obj)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.CacheObject{
		Data:     buf.Bytes(),
		Metadata: info,
	}, nil
}

func (sp *s3StorageProvider) Query(
	ctx context.Context,
	keys []*types.CacheKey,
) ([]*types.CacheObjectMeta, error) {
	sp.knownObjectsMutex.RLock()
	defer sp.knownObjectsMutex.RUnlock()

	results := make([]*types.CacheObjectMeta, len(keys))
	for i, key := range keys {
		if meta, ok := sp.knownObjects[key.GetHash()]; ok {
			results[i] = meta
		}
	}
	return results, nil
}

func (sp *s3StorageProvider) UsageInfo() *metrics.UsageInfo {
	sp.knownObjectsMutex.RLock()
	defer sp.knownObjectsMutex.RUnlock()

	totalSize := sp.totalSize.Load()
	var usagePercent float64
	if sp.storageLimitBytes == 0 {
		usagePercent = 0
	} else {
		usagePercent = float64(totalSize) / float64(sp.storageLimitBytes)
	}
	return &metrics.UsageInfo{
		ObjectCount:  int64(len(sp.knownObjects)),
		TotalSize:    totalSize,
		UsagePercent: usagePercent,
	}
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
