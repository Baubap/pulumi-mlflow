package client

import (
	"context"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// Config holds the MLflow provider configuration. It is registered with the
// provider and hydrated once per process; resources retrieve it via
// infer.GetConfig[client.Config](ctx) and call Client().
type Config struct {
	TrackingURI        string `pulumi:"trackingUri,optional"`
	Username           string `pulumi:"username,optional"`
	Password           string `pulumi:"password,optional" provider:"secret"`
	Token              string `pulumi:"token,optional" provider:"secret"`
	InsecureSkipVerify bool   `pulumi:"insecureSkipVerify,optional"`
	// PluginDownloadURL is where the engine downloaded this provider from. The
	// engine adds it to a default provider's inputs; declaring it keeps it in
	// the provider's diff on both sides, so an unchanged URL is not a change.
	PluginDownloadURL string `pulumi:"pluginDownloadURL,optional"`

	client *Client
}

var (
	_ infer.CustomConfigure              = (*Config)(nil)
	_ infer.CustomDiff[*Config, *Config] = (*Config)(nil)
)

// Diff replaces the provider, and every resource it manages, only when the
// MLflow connection changes. PluginDownloadURL is where the engine downloaded
// the plugin from, not a setting of the connection: the engine sets it on one
// side and not the other when a stack moves between a pluginDownloadURL
// resource option and the SDK's default, and that must not replace anything.
// A new provider version is diffed by infer after this, as an in-place update.
func (c *Config) Diff(_ context.Context, req infer.DiffRequest[*Config, *Config]) (infer.DiffResponse, error) {
	olds, news := req.State, req.Inputs
	if olds == nil {
		olds = &Config{}
	}
	if news == nil {
		news = &Config{}
	}
	changed := map[string]bool{
		"trackingUri":        olds.TrackingURI != news.TrackingURI,
		"username":           olds.Username != news.Username,
		"password":           olds.Password != news.Password,
		"token":              olds.Token != news.Token,
		"insecureSkipVerify": olds.InsecureSkipVerify != news.InsecureSkipVerify,
	}
	resp := infer.DiffResponse{DetailedDiff: map[string]p.PropertyDiff{}}
	for key, isChanged := range changed {
		if isChanged {
			resp.DetailedDiff[key] = p.PropertyDiff{Kind: p.UpdateReplace, InputDiff: true}
			resp.HasChanges = true
		}
	}
	return resp, nil
}

// Annotate documents the config fields and wires environment-variable fallbacks.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.TrackingURI, "URL of the MLflow tracking server, e.g. https://mlflow.example.com. "+
		"May also be set via the MLFLOW_TRACKING_URI environment variable.")
	a.Describe(&c.Username, "Username for HTTP Basic authentication against the MLflow tracking server.")
	a.Describe(&c.Password, "Password for HTTP Basic authentication.")
	a.Describe(&c.Token, "Bearer token for authenticating against the MLflow tracking server. "+
		"Takes precedence over username/password.")
	a.Describe(&c.PluginDownloadURL, "Where the engine downloaded this provider plugin from. Set by the engine; "+
		"not a setting of the MLflow connection.")
	a.Describe(&c.InsecureSkipVerify, "Skip TLS certificate verification when connecting to the tracking "+
		"server. Not recommended in production.")

	a.SetDefault(&c.TrackingURI, "", "MLFLOW_TRACKING_URI")
	a.SetDefault(&c.Username, "", "MLFLOW_TRACKING_USERNAME")
	a.SetDefault(&c.Password, "", "MLFLOW_TRACKING_PASSWORD")
	a.SetDefault(&c.Token, "", "MLFLOW_TRACKING_TOKEN")
}

// Configure builds the shared MLflow client from the resolved configuration.
func (c *Config) Configure(_ context.Context) error {
	cl, err := NewClient(c.TrackingURI, c.Username, c.Password, c.Token, c.InsecureSkipVerify)
	if err != nil {
		return err
	}
	c.client = cl
	return nil
}

// Client returns the configured MLflow client.
func (c Config) Client() *Client { return c.client }
