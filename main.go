// Command terraform-provider-netboxplugins serves the netboxplugins provider.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/plugins"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/provider"
)

// version is set by the release build.
var version = "dev"

//go:generate go tool tfplugindocs generate --provider-name netboxplugins

func main() {
	debug := flag.Bool("debug", false, "start the provider in debug mode")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version, plugins.All), providerserver.ServeOpts{
		Address: "registry.terraform.io/gluendo/netboxplugins",
		Debug:   *debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
