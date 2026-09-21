// Package provider implements the Terraform Plugin Framework provider for
// jmnote/wslc. It depends on internal/wslc for every actual interaction
// with wslc.exe; nothing in internal/wslc imports this package or
// terraform-plugin-framework, which is what keeps internal/wslc unit
// testable without Terraform or a real WSL container host.
package provider

import (
	"context"
	"runtime"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/jmnote/terraform-provider-wslc/internal/wslc"
)

// Ensure WslcProvider satisfies the expected interfaces.
var _ provider.Provider = &WslcProvider{}

// WslcProvider is the jmnote/wslc Terraform provider. It manages the
// lifecycle of WSL containers on the local Windows host that runs
// Terraform, via wslc.exe (the WSL Container CLI).
type WslcProvider struct {
	// version is set by main.go via goreleaser at build time (or "dev"
	// for local builds) and reported in provider metadata.
	version string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &WslcProvider{version: version}
	}
}

func (p *WslcProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "wslc"
	resp.Version = p.version
}

func (p *WslcProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages Linux containers under the Windows Subsystem for Linux (WSL) container feature, " +
			"via wslc.exe. This provider must run as part of Terraform executing on a Windows host with the " +
			"WSL container feature available (WSL 2.9.3 or later); it does not manage anything inside a " +
			"container beyond what wslc.exe itself exposes.",
		Attributes: map[string]schema.Attribute{},
	}
}

func (p *WslcProvider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	if runtime.GOOS != "windows" {
		resp.Diagnostics.AddError(
			"Unsupported host operating system",
			"The wslc provider drives the wslc.exe command-line tool and only runs as part of Terraform "+
				"executing on Windows. Detected host OS: \""+runtime.GOOS+"\".",
		)
		return
	}

	// wslc.exe ships as part of the WSL container feature and is always
	// resolved via PATH; there is no supported scenario where it lives
	// somewhere else, so the executable to invoke is not configurable.
	// A --session override was tried and dropped: wslc.exe's session
	// feature is still pre-release and its behavior (e.g. which
	// identifier --session actually accepts) is not settled enough to
	// commit to in this provider's schema yet.
	client := wslc.NewClient(wslc.NewProcessRunner(""))
	resp.ResourceData = client
	resp.DataSourceData = client
}

func (p *WslcProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewContainerResource,
	}
}

func (p *WslcProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}
