// Package version holds the provider version, injected by the linker at build time.
package version

// Version is the semantic version of this provider build. It is set via
// -ldflags "-X github.com/Baubap/pulumi-mlflow/provider/version.Version=vX.Y.Z".
var Version string
