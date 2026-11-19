package registry

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

const artifactsRoute = "/api/2.0/mlflow-artifacts/artifacts"

// TestCopyArtifactsProxy exercises the mlflow-artifacts:/ backend end to end
// against fake proxy servers: recursive list + download from the source, upload
// to the destination — no cloud credentials, no S3.
func TestCopyArtifactsProxy(t *testing.T) {
	// Source tree under "model/": MLmodel, data/x.bin (exercises recursion).
	srcFiles := map[string]string{
		"model/MLmodel":    "flavors: {}\n",
		"model/data/x.bin": "\x00\x01\x02",
	}
	srcMux := http.NewServeMux()
	srcMux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("2.16.2")) })
	srcMux.HandleFunc(artifactsRoute, func(w http.ResponseWriter, r *http.Request) { // list (non-recursive)
		dir := strings.Trim(r.URL.Query().Get("path"), "/")
		var files []client.ArtifactFile
		seenDir := map[string]bool{}
		for p := range srcFiles {
			if dir != "" && !strings.HasPrefix(p, dir+"/") {
				continue
			}
			rest := strings.TrimPrefix(strings.TrimPrefix(p, dir), "/")
			if i := strings.IndexByte(rest, '/'); i >= 0 { // child is a directory
				sub := dir + "/" + rest[:i]
				sub = strings.TrimPrefix(sub, "/")
				if !seenDir[sub] {
					seenDir[sub] = true
					files = append(files, client.ArtifactFile{Path: sub, IsDir: true})
				}
				continue
			}
			files = append(files, client.ArtifactFile{Path: p, IsDir: false})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"files": files})
	})
	srcMux.HandleFunc(artifactsRoute+"/", func(w http.ResponseWriter, r *http.Request) { // download
		p := strings.TrimPrefix(r.URL.Path, artifactsRoute+"/")
		body, ok := srcFiles[p]
		if !ok {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	})
	srcSrv := httptest.NewServer(srcMux)
	defer srcSrv.Close()

	// Destination collects uploads.
	var mu sync.Mutex
	got := map[string]string{}
	dstMux := http.NewServeMux()
	dstMux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("2.16.2")) })
	dstMux.HandleFunc(artifactsRoute+"/", func(w http.ResponseWriter, r *http.Request) { // upload
		p := strings.TrimPrefix(r.URL.Path, artifactsRoute+"/")
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got[p] = string(b)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	dstSrv := httptest.NewServer(dstMux)
	defer dstSrv.Close()

	srcClient, err := client.NewClient(srcSrv.URL, "", "", "", false)
	require.NoError(t, err)
	dstClient, err := client.NewClient(dstSrv.URL, "", "", "", false)
	require.NoError(t, err)

	err = copyArtifacts(context.Background(), srcClient, dstClient, "mlflow-artifacts:/model", "mlflow-artifacts:/dst/model")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, "flavors: {}\n", got["dst/model/MLmodel"])
	assert.Equal(t, "\x00\x01\x02", got["dst/model/data/x.bin"])
	assert.Len(t, got, 2)
}
