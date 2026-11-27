package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// R2Options configures an R2Store.
type R2Options struct {
	AccountID       string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	// Endpoint overrides https://<AccountID>.r2.cloudflarestorage.com.
	Endpoint string
	// PublicBaseURL is an optional https base (custom domain or r2.dev) that
	// serves the bucket publicly.
	PublicBaseURL string
}

// R2Store stores objects in a Cloudflare R2 bucket through the S3 API.
type R2Store struct {
	client     *minio.Client
	bucket     string
	publicBase string
}

// R2EndpointHost returns the S3 API host for the options.
func R2EndpointHost(opts R2Options) (string, error) {
	raw := strings.TrimSpace(opts.Endpoint)
	if raw == "" {
		account := strings.TrimSpace(opts.AccountID)
		if account == "" || strings.ContainsAny(account, "/:@?#. ") {
			return "", errors.New("storage: R2_ACCOUNT_ID is required (or R2_ENDPOINT)")
		}
		return account + ".r2.cloudflarestorage.com", nil
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("storage: invalid R2_ENDPOINT %q", opts.Endpoint)
	}
	if u.Scheme != "https" {
		return "", errors.New("storage: R2_ENDPOINT must use https")
	}
	if u.Path != "" && u.Path != "/" {
		return "", errors.New("storage: R2_ENDPOINT must not contain a path")
	}
	return u.Host, nil
}

// NewR2 builds an R2 client. It does not contact the network.
func NewR2(opts R2Options) (*R2Store, error) {
	if strings.TrimSpace(opts.Bucket) == "" {
		return nil, errors.New("storage: R2_BUCKET is required")
	}
	if opts.AccessKeyID == "" || opts.SecretAccessKey == "" {
		return nil, errors.New("storage: R2_ACCESS_KEY_ID and R2_SECRET_ACCESS_KEY are required")
	}
	host, err := R2EndpointHost(opts)
	if err != nil {
		return nil, err
	}
	publicBase := strings.TrimRight(strings.TrimSpace(opts.PublicBaseURL), "/")
	if publicBase != "" {
		u, err := url.Parse(publicBase)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("storage: R2_PUBLIC_BASE_URL must be an absolute https URL")
		}
	}
	client, err := minio.New(host, &minio.Options{
		Creds:        credentials.NewStaticV4(opts.AccessKeyID, opts.SecretAccessKey, ""),
		Secure:       true,
		Region:       "auto",
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("storage: R2 client: %w", err)
	}
	return &R2Store{client: client, bucket: opts.Bucket, publicBase: publicBase}, nil
}

func mapR2Error(err error) error {
	if err == nil {
		return nil
	}
	resp := minio.ToErrorResponse(err)
	switch {
	case resp.Code == "NoSuchKey", resp.Code == "NotFound", resp.StatusCode == http.StatusNotFound && resp.Code != "NoSuchBucket":
		return ErrNotFound
	}
	return err
}

func objectKey(key string) (string, error) {
	clean, err := Key(key)
	if err != nil || clean != key {
		return "", ErrInvalidKey
	}
	return clean, nil
}

func toInfo(oi minio.ObjectInfo) ObjectInfo {
	return ObjectInfo{
		Key:         oi.Key,
		Size:        oi.Size,
		ContentType: oi.ContentType,
		ModTime:     oi.LastModified,
		ETag:        strings.Trim(oi.ETag, "\""),
	}
}

func (s *R2Store) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	k, err := objectKey(key)
	if err != nil {
		return err
	}
	if contentType == "" {
		contentType = ContentTypeFor(k)
	}
	_, err = s.client.PutObject(ctx, s.bucket, k, r, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (s *R2Store) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	k, err := objectKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, k, minio.GetObjectOptions{})
	if err != nil {
		return nil, ObjectInfo{}, mapR2Error(err)
	}
	st, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, ObjectInfo{}, mapR2Error(err)
	}
	info := toInfo(st)
	info.Key = k
	return obj, info, nil
}

func (s *R2Store) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	k, err := objectKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	st, err := s.client.StatObject(ctx, s.bucket, k, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, mapR2Error(err)
	}
	info := toInfo(st)
	info.Key = k
	return info, nil
}

func (s *R2Store) Delete(ctx context.Context, key string) error {
	k, err := objectKey(key)
	if err != nil {
		return err
	}
	if err := mapR2Error(s.client.RemoveObject(ctx, s.bucket, k, minio.RemoveObjectOptions{})); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}

// PublicURL returns R2_PUBLIC_BASE_URL joined with the escaped key.
func (s *R2Store) PublicURL(key string) (string, bool) {
	if s.publicBase == "" {
		return "", false
	}
	k, err := objectKey(key)
	if err != nil {
		return "", false
	}
	return joinPublicURL(s.publicBase, k), true
}

func joinPublicURL(base, key string) string {
	segs := strings.Split(key, "/")
	for i, seg := range segs {
		segs[i] = url.PathEscape(seg)
	}
	return base + "/" + strings.Join(segs, "/")
}

func (s *R2Store) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	p, err := Key(prefix)
	if err != nil {
		return nil, err
	}
	var out []ObjectInfo
	for oi := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: p + "/", Recursive: true}) {
		if oi.Err != nil {
			return nil, mapR2Error(oi.Err)
		}
		out = append(out, toInfo(oi))
	}
	return out, nil
}
