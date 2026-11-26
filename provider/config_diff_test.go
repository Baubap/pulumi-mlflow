package provider_test

import (
	"context"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	mlflow "github.com/Baubap/pulumi-mlflow/provider"
)

const (
	trackingURI       = "https://baubap-ml-mlflow-testing.baubap.com"
	pluginDownloadURL = "https://github.com/Baubap/pulumi-mlflow/releases/download/v${VERSION}"
)

// defaultProviderInputs are a default provider's inputs as the engine sends
// them: the stack's config plus the engine's version and pluginDownloadURL.
func defaultProviderInputs(uri, version string) property.Map {
	return property.NewMap(map[string]property.Value{
		"trackingUri":       property.New(uri),
		"version":           property.New(version),
		"pluginDownloadURL": property.New(pluginDownloadURL),
	})
}

func diffConfig(t *testing.T, olds, news property.Map) p.DiffResponse {
	t.Helper()
	server, err := integration.NewServer(context.Background(), "mlflow", semver.MustParse("0.2.3"),
		integration.WithProvider(mlflow.Provider()))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.DiffConfig(p.DiffRequest{
		Urn:       resource.URN("urn:pulumi:test::proj::pulumi:providers:mlflow::default"),
		State:     olds,
		OldInputs: olds,
		Inputs:    news,
	})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func replaces(resp p.DiffResponse) bool {
	for _, d := range resp.DetailedDiff {
		switch d.Kind {
		case p.AddReplace, p.UpdateReplace, p.DeleteReplace:
			return true
		}
	}
	return false
}

func TestUnchangedConfigIsNotAChange(t *testing.T) {
	inputs := defaultProviderInputs(trackingURI, "0.2.3")
	if resp := diffConfig(t, inputs, inputs); resp.HasChanges {
		t.Fatalf("unchanged provider config reported a change: %+v", resp.DetailedDiff)
	}
}

func TestNewProviderVersionUpdatesInPlace(t *testing.T) {
	resp := diffConfig(t, defaultProviderInputs(trackingURI, "0.2.2"), defaultProviderInputs(trackingURI, "0.2.3"))
	if !resp.HasChanges || replaces(resp) {
		t.Fatalf("a new provider version must update the provider in place: %+v", resp.DetailedDiff)
	}
}

func TestNewTrackingServerReplacesProvider(t *testing.T) {
	resp := diffConfig(t, defaultProviderInputs(trackingURI, "0.2.3"),
		defaultProviderInputs("https://baubap-ml-mlflow-production.baubap.com", "0.2.3"))
	if !replaces(resp) {
		t.Fatalf("a different tracking server must replace the provider: %+v", resp.DetailedDiff)
	}
}

// The engine diffs a stack's old default provider against its new one with
// their inputs, and pluginDownloadURL is in them only when the stack set it as
// a resource option. A stack that drops the option for the SDK's default, the
// same URL, must not replace the provider and every resource with it.
func TestPluginDownloadURLIsNotAConfigChange(t *testing.T) {
	withURL := defaultProviderInputs(trackingURI, "0.2.3")
	withoutURL := property.NewMap(map[string]property.Value{
		"trackingUri": property.New(trackingURI),
		"version":     property.New("0.2.4"),
	})
	for name, pair := range map[string][2]property.Map{
		"dropped": {withURL, withoutURL},
		"added":   {withoutURL, defaultProviderInputs(trackingURI, "0.2.4")},
		"changed": {withURL, property.NewMap(map[string]property.Value{
			"trackingUri":       property.New(trackingURI),
			"version":           property.New("0.2.4"),
			"pluginDownloadURL": property.New("github://api.github.com/Baubap/pulumi-mlflow"),
		})},
	} {
		if resp := diffConfig(t, pair[0], pair[1]); replaces(resp) {
			t.Errorf("%s pluginDownloadURL replaced the provider: %+v", name, resp.DetailedDiff)
		}
	}
}
