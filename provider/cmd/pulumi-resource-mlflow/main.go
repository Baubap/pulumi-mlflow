// Package main runs the MLflow provider against Pulumi's provider protocol.
package main

import (
	"context"
	"fmt"
	"os"

	provider "github.com/Baubap/pulumi-mlflow/provider"
	"github.com/Baubap/pulumi-mlflow/provider/version"
)

func main() {
	err := provider.Provider().Run(context.Background(), provider.Name, version.Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
