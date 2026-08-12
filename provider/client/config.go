package client

import (
	"context"

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

	client *Client
}

var _ infer.CustomConfigure = (*Config)(nil)

// Annotate documents the config fields and wires environment-variable fallbacks.
func (c *Config) Annotate(a infer.Annotator) {
	a.Describe(&c.TrackingURI, "URL of the MLflow tracking server, e.g. https://mlflow.example.com. "+
		"May also be set via the MLFLOW_TRACKING_URI environment variable.")
	a.Describe(&c.Username, "Username for HTTP Basic authentication against the MLflow tracking server.")
	a.Describe(&c.Password, "Password for HTTP Basic authentication.")
	a.Describe(&c.Token, "Bearer token for authenticating against the MLflow tracking server. "+
		"Takes precedence over username/password.")
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
