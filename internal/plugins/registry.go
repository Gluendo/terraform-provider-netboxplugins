// Package plugins lists the plugin folders compiled into the provider.
package plugins

import (
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/core/model"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/plugins/dataflows"
	"github.com/Gluendo/terraform-provider-netboxplugins/internal/plugins/dns"
)

// All is every plugin folder, in resource-name order.
var All = []*model.Plugin{
	dataflows.Plugin,
	dns.Plugin,
}
