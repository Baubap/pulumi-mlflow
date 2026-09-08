package registry

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// copyArtifactsFn is the artifact-copy seam (overridable in tests).
var copyArtifactsFn = copyArtifacts

// copyArtifacts copies every artifact under srcURI to dstURI, dispatching on the
// URI scheme like the MLflow client. Supported:
//   - s3:// on both sides: direct server-side S3 CopyObject (bytes never transit
//     the process; needs credentials + access to both buckets).
//   - mlflow-artifacts:/ : streamed through each tracking server's artifact REST
//     API (no cloud credentials). srcClient serves the source side, dstClient the
//     destination. Mixed s3:// <-> mlflow-artifacts:/ is supported by streaming
//     each file through the process.
func copyArtifacts(ctx context.Context, srcClient, dstClient *client.Client, srcURI, dstURI string) error {
	srcScheme, dstScheme := uriScheme(srcURI), uriScheme(dstURI)
	if !supportedScheme(srcScheme) || !supportedScheme(dstScheme) {
		return fmt.Errorf(
			"artifact copy for scheme %q -> %q is not supported (supported: s3://, mlflow-artifacts:/)",
			srcScheme, dstScheme)
	}
	if srcScheme == "s3" && dstScheme == "s3" {
		return copyArtifactsS3(ctx, srcURI, dstURI) // fast path: bytes stay in S3
	}

	// Generic per-file streaming copy (mlflow-artifacts:/ and mixed).
	var s3c *s3.Client
	if srcScheme == "s3" || dstScheme == "s3" {
		c, err := newS3Client(ctx)
		if err != nil {
			return err
		}
		s3c = c
	}
	rels, err := listArtifactFiles(ctx, srcClient, s3c, srcURI)
	if err != nil {
		return err
	}
	if len(rels) == 0 {
		return fmt.Errorf("no artifacts found under %s", srcURI)
	}
	for _, rel := range rels {
		data, err := readArtifactFile(ctx, srcClient, s3c, srcURI, rel)
		if err != nil {
			return err
		}
		if err := writeArtifactFile(ctx, dstClient, s3c, dstURI, rel, data); err != nil {
			return err
		}
	}
	return nil
}

func supportedScheme(s string) bool { return s == "s3" || s == "mlflow-artifacts" }

func uriScheme(uri string) string {
	if i := strings.IndexByte(uri, ':'); i > 0 {
		return uri[:i]
	}
	return ""
}

// proxyPath returns the artifact-root-relative path of an mlflow-artifacts:/ URI
// (dropping the scheme and any host).
func proxyPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(strings.TrimPrefix(uri, "mlflow-artifacts:"), "/")
	}
	return strings.TrimPrefix(u.Path, "/")
}

// ---- generic per-file backend ------------------------------------------------

func listArtifactFiles(ctx context.Context, cl *client.Client, s3c *s3.Client, srcURI string) ([]string, error) {
	switch uriScheme(srcURI) {
	case "s3":
		return listS3Files(ctx, s3c, srcURI)
	case "mlflow-artifacts":
		return listProxyFiles(ctx, cl, proxyPath(srcURI))
	default:
		return nil, fmt.Errorf("unsupported source scheme in %q", srcURI)
	}
}

func readArtifactFile(ctx context.Context, cl *client.Client, s3c *s3.Client, srcURI, rel string) ([]byte, error) {
	switch uriScheme(srcURI) {
	case "s3":
		bucket, key, err := parseS3(srcURI)
		if err != nil {
			return nil, err
		}
		return getS3Object(ctx, s3c, bucket, joinKey(key, rel))
	case "mlflow-artifacts":
		return cl.DownloadArtifactProxy(ctx, joinKey(proxyPath(srcURI), rel))
	default:
		return nil, fmt.Errorf("unsupported source scheme in %q", srcURI)
	}
}

func writeArtifactFile(ctx context.Context, cl *client.Client, s3c *s3.Client, dstURI, rel string, data []byte) error {
	switch uriScheme(dstURI) {
	case "s3":
		bucket, key, err := parseS3(dstURI)
		if err != nil {
			return err
		}
		return putS3Object(ctx, s3c, bucket, joinKey(key, rel), data)
	case "mlflow-artifacts":
		return cl.UploadArtifactProxy(ctx, joinKey(proxyPath(dstURI), rel), data)
	default:
		return fmt.Errorf("unsupported destination scheme in %q", dstURI)
	}
}

func joinKey(base, rel string) string {
	base = strings.TrimSuffix(base, "/")
	if rel == "" {
		return base
	}
	return base + "/" + rel
}

// listProxyFiles recursively lists file paths (relative to root) under an
// artifact-proxy root path.
func listProxyFiles(ctx context.Context, cl *client.Client, root string) ([]string, error) {
	var files []string
	var walk func(dir string) error
	walk = func(dir string) error {
		entries, err := cl.ListArtifactsProxy(ctx, dir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir {
				if err := walk(e.Path); err != nil {
					return err
				}
				continue
			}
			files = append(files, strings.TrimPrefix(strings.TrimPrefix(e.Path, root), "/"))
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	return files, nil
}

// ---- S3 backend --------------------------------------------------------------

func newS3Client(ctx context.Context) (*s3.Client, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}
	// A custom endpoint (AWS_ENDPOINT_URL_S3) is honored by LoadDefaultConfig; for
	// S3-compatible stores like MinIO/LocalStack used in e2e, path-style addressing
	// is also required — opt in via AWS_S3_FORCE_PATH_STYLE=true. No effect on real S3.
	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		if os.Getenv("AWS_S3_FORCE_PATH_STYLE") == "true" {
			o.UsePathStyle = true
		}
	}), nil
}

// parseS3 splits an s3://bucket/key URI into bucket and key.
func parseS3(uri string) (bucket, key string, err error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "s3" || u.Host == "" {
		return "", "", fmt.Errorf("invalid s3 URI %q", uri)
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}

func listS3Files(ctx context.Context, s3c *s3.Client, srcURI string) ([]string, error) {
	bucket, key, err := parseS3(srcURI)
	if err != nil {
		return nil, err
	}
	prefix := strings.TrimSuffix(key, "/")
	var rels []string
	paginator := s3.NewListObjectsV2Paginator(s3c, &s3.ListObjectsV2Input{Bucket: &bucket, Prefix: &prefix})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing s3://%s/%s: %w", bucket, prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			rels = append(rels, strings.TrimPrefix(strings.TrimPrefix(*obj.Key, prefix), "/"))
		}
	}
	return rels, nil
}

func getS3Object(ctx context.Context, s3c *s3.Client, bucket, key string) ([]byte, error) {
	out, err := s3c.GetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key})
	if err != nil {
		return nil, fmt.Errorf("get s3://%s/%s: %w", bucket, key, err)
	}
	defer func() { _ = out.Body.Close() }()
	return io.ReadAll(out.Body)
}

func putS3Object(ctx context.Context, s3c *s3.Client, bucket, key string, data []byte) error {
	_, err := s3c.PutObject(ctx, &s3.PutObjectInput{Bucket: &bucket, Key: &key, Body: strings.NewReader(string(data))})
	if err != nil {
		return fmt.Errorf("put s3://%s/%s: %w", bucket, key, err)
	}
	return nil
}

// copyArtifactsS3 recursively server-side-copies all objects under the source
// prefix to the destination prefix (same-account CopyObject keeps bytes in S3).
func copyArtifactsS3(ctx context.Context, srcURI, dstURI string) error {
	srcBucket, srcKey, err := parseS3(srcURI)
	if err != nil {
		return err
	}
	dstBucket, dstKey, err := parseS3(dstURI)
	if err != nil {
		return err
	}
	s3c, err := newS3Client(ctx)
	if err != nil {
		return err
	}

	prefix := strings.TrimSuffix(srcKey, "/")
	paginator := s3.NewListObjectsV2Paginator(s3c, &s3.ListObjectsV2Input{Bucket: &srcBucket, Prefix: &prefix})
	copied := 0
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("listing s3://%s/%s: %w", srcBucket, prefix, err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			rel := strings.TrimPrefix(strings.TrimPrefix(*obj.Key, prefix), "/")
			destKey := joinKey(dstKey, rel)
			copySource := (&url.URL{Path: srcBucket + "/" + *obj.Key}).EscapedPath()
			if _, err := s3c.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:     &dstBucket,
				Key:        &destKey,
				CopySource: &copySource,
			}); err != nil {
				return fmt.Errorf("copying s3://%s/%s -> s3://%s/%s: %w", srcBucket, *obj.Key, dstBucket, destKey, err)
			}
			copied++
		}
	}
	if copied == 0 {
		return fmt.Errorf("no artifacts found under %s", srcURI)
	}
	return nil
}
