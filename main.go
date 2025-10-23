package main

import (
	"context"
	"flag"
	"log"

	"github.com/andredelgadoruiz/terraform-provider-gorules/internal/provider"
	frameworkProvider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// These are set during build time via ldflags
var (
	version = "dev"
	commit  = ""
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/andredelgadoruiz/gorules",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), func() frameworkProvider.Provider {
		return provider.New(version)
	}, opts)
	if err != nil {
		log.Fatal(err.Error())
	}
}
