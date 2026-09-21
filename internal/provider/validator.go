package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// Preserve host port zero for CLI-specific allocation behavior. Container ports
// must be positive. The CLI remains responsible for platform-specific constraints.
type portValidator struct{ minimum int64 }

func (v portValidator) Description(context.Context) string {
	return fmt.Sprintf("Port must be between %d and 65535.", v.minimum)
}
func (v portValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (v portValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	n := req.ConfigValue.ValueInt64()
	if n < v.minimum || n > 65535 {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid port", v.Description(ctx))
	}
}

type protocolValidator struct{}

func (protocolValidator) Description(context.Context) string               { return "Protocol must be tcp or udp." }
func (v protocolValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (v protocolValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	p := req.ConfigValue.ValueString()
	if p != "tcp" && p != "udp" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid protocol", v.Description(ctx))
	}
}
