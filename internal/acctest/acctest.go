// Package acctest holds what every plugin folder's acceptance tests share
// (ADR-0007): provider factories, environment checks, an API client for
// changes made outside Terraform, and the standard drift steps.
package acctest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/client"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/plugins"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/provider"
)

// CoreProviderVersion is the e-breuninger/netbox version the tests use for
// core objects (ADR-0004).
const CoreProviderVersion = "6.0.1"

// CoreProviderBlock declares the core provider in a test configuration.
// Configurations carry it themselves because import steps do not inherit
// the test case's external providers.
const CoreProviderBlock = `
terraform {
  required_providers {
    netbox = {
      source  = "e-breuninger/netbox"
      version = "` + CoreProviderVersion + `"
    }
  }
}
`

// Factories serves the provider under test.
func Factories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		provider.TypeName: providerserver.NewProtocol6WithError(provider.New("acctest", plugins.All)()),
	}
}

// CoreProvider is the external core NetBox provider.
func CoreProvider() map[string]resource.ExternalProvider {
	return map[string]resource.ExternalProvider{
		"netbox": {Source: "e-breuninger/netbox", VersionConstraint: CoreProviderVersion},
	}
}

// PreCheck fails the test unless a NetBox test environment is configured.
func PreCheck(t *testing.T) {
	t.Helper()
	for _, v := range []string{"NETBOX_SERVER_URL", "NETBOX_API_TOKEN"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set; start an environment with testenv/up.sh", v)
		}
	}
}

// API returns a client for changes made outside Terraform. Close it after use.
func API(t *testing.T) *client.Client {
	t.Helper()
	c, err := client.New(client.Config{
		ServerURL:      os.Getenv("NETBOX_SERVER_URL"),
		APIToken:       os.Getenv("NETBOX_API_TOKEN"),
		RequestTimeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Name returns a unique name for test objects.
func Name(prefix string) string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(b))
}

// Octet returns a host octet in 10..249, so test addresses differ between runs.
func Octet() int {
	b := make([]byte, 1)
	_, _ = rand.Read(b)
	return 10 + int(b[0])%240
}

// Patch changes an object through the API, as a user would outside
// Terraform.
func Patch(t *testing.T, path string, body map[string]any) {
	t.Helper()
	c := API(t)
	defer c.Close()
	if _, err := c.Patch(context.Background(), path, body); err != nil {
		t.Fatalf("patch %s: %v", path, err)
	}
}

// DriftSteps are the standard steps after a create (ADR-0007): change the
// object through the API, expect the plan to show it, then apply and expect
// the configuration restored. check runs after the restore.
func DriftSteps(config string, change func(), check resource.TestCheckFunc) []resource.TestStep {
	return []resource.TestStep{
		{
			PreConfig:          change,
			Config:             config,
			PlanOnly:           true,
			ExpectNonEmptyPlan: true,
		},
		{
			Config: config,
			Check:  check,
		},
	}
}
