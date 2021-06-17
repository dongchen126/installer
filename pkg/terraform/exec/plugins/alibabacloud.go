package plugins

import (
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
)

func init() {
	// TODO AlibabaCloud: A later PR
	// "github.com/terraform-providers/terraform-provider-alicloud/alicloud"
	exec := func() {
		plugin.Serve(&plugin.ServeOpts{
			// ProviderFunc: alicloud.Provider,
		})
	}
	KnownPlugins["terraform-provider-alicloud"] = exec
}
