package registry

import (
	"context"
	"fmt"
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
// URI scheme like the MLflow client does. v1 supports s3:// (direct, via the AWS
// SDK — the process must have credentials + access to both buckets). Other
// schemes (mlflow-artifacts:/ proxy, gs://, wasbs://, …) are not implemented yet
// and return a clear error.
//
// api is the destination MLflow client, used by the (future) mlflow-artifacts:/
// proxy backend; the s3:// backend talks to S3 directly and ignores it.
func copyArtifacts(ctx context.Context, api *client.Client, srcURI, dstURI string) error {
	srcScheme := uriScheme(srcURI)
	dstScheme := uriScheme(dstURI)
	switch {
	case srcScheme == "s3" && dstScheme == "s3":
		return copyArtifactsS3(ctx, srcURI, dstURI)
	default:
		return fmt.Errorf(
			"artifact copy for scheme %q -> %q is not supported yet (v1 supports s3://). "+
				"Enable proxied artifacts (--serve-artifacts) or extend the artifact backend",
			srcScheme, dstScheme)
	}
}

func uriScheme(uri string) string {
	if i := strings.Index(uri, "://"); i > 0 {
		return uri[:i]
	}
	return ""
}

// parseS3 splits an s3://bucket/key URI into bucket and key.
func parseS3(uri string) (bucket, key string, err error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "s3" || u.Host == "" {
		return "", "", fmt.Errorf("invalid s3 URI %q", uri)
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}

// copyArtifactsS3 recursively server-side-copies all objects under the source
// prefix to the destination prefix. Same-account CopyObject keeps bytes inside
// S3 (nothing streams through the process). Idempotent: re-copying overwrites.
func copyArtifactsS3(ctx context.Context, srcURI, dstURI string) error {
	srcBucket, srcKey, err := parseS3(srcURI)
	if err != nil {
		return err
	}
	dstBucket, dstKey, err := parseS3(dstURI)
	if err != nil {
		return err
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("loading AWS config: %w", err)
	}
	// A custom endpoint (AWS_ENDPOINT_URL_S3) is honored by LoadDefaultConfig; for
	// S3-compatible stores like MinIO/LocalStack used in e2e, path-style addressing
	// is also required — opt in via AWS_S3_FORCE_PATH_STYLE=true. No effect on real S3.
	s3c := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if os.Getenv("AWS_S3_FORCE_PATH_STYLE") == "true" {
			o.UsePathStyle = true
		}
	})

	prefix := strings.TrimSuffix(srcKey, "/")
	paginator := s3.NewListObjectsV2Paginator(s3c, &s3.ListObjectsV2Input{
		Bucket: &srcBucket,
		Prefix: &prefix,
	})
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
			destKey := strings.TrimSuffix(dstKey, "/")
			if rel != "" {
				destKey += "/" + rel
			}
			// CopySource must be a URL-encoded "bucket/key" (keeps '/').
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
