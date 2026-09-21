package provider

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/jmnote/terraform-provider-wslc/internal/wslc"
)

func testPlan(t *testing.T, overrides map[string]tftypes.Value) tfsdk.Plan {
	t.Helper()
	ctx := context.Background()
	var resp resource.SchemaResponse
	(&containerResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	typ := resp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	values := map[string]tftypes.Value{}
	for k, v := range typ.AttributeTypes {
		values[k] = tftypes.NewValue(v, nil)
	}
	values["name"] = tftypes.NewValue(tftypes.String, "test")
	values["image"] = tftypes.NewValue(tftypes.String, "nginx")
	values["id"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	values["state"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	for k, v := range overrides {
		values[k] = v
	}
	return tfsdk.Plan{Schema: resp.Schema, Raw: tftypes.NewValue(typ, values)}
}

func TestValidateUnknownCollections(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value tftypes.Value
	}{
		{"env", tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{"TOKEN": tftypes.NewValue(tftypes.String, tftypes.UnknownValue)})},
		{"labels", tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, tftypes.UnknownValue)},
		{"command", tftypes.NewValue(tftypes.List{ElementType: tftypes.String}, tftypes.UnknownValue)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := testPlan(t, map[string]tftypes.Value{tc.name: tc.value})
			var resp resource.ValidateConfigResponse
			(&containerResource{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: p.Schema, Raw: p.Raw}}, &resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}
			var model containerResourceModel
			if d := p.Get(context.Background(), &model); d.HasError() {
				t.Fatal(d)
			}
		})
	}
}

func TestValidatePull(t *testing.T) {
	for _, value := range []string{"always", "missing", "never", "", "invalid"} {
		t.Run(value, func(t *testing.T) {
			p := testPlan(t, map[string]tftypes.Value{"pull": tftypes.NewValue(tftypes.String, value)})
			var resp resource.ValidateConfigResponse
			(&containerResource{}).ValidateConfig(context.Background(), resource.ValidateConfigRequest{Config: tfsdk.Config{Schema: p.Schema, Raw: p.Raw}}, &resp)
			if resp.Diagnostics.HasError() != (value == "" || value == "invalid") {
				t.Fatal(resp.Diagnostics)
			}
		})
	}
}

type fakeClient struct {
	startErr, inspectErr error
	opts                 wslc.CreateContainerOptions
}

func (c *fakeClient) CreateContainer(_ context.Context, opts wslc.CreateContainerOptions) (string, error) {
	c.opts = opts
	return "id123", nil
}
func (c *fakeClient) StartContainer(context.Context, string) error  { return c.startErr }
func (c *fakeClient) RemoveContainer(context.Context, string) error { return nil }
func (c *fakeClient) InspectContainer(context.Context, string) (*wslc.ContainerInspect, error) {
	if c.inspectErr != nil {
		return nil, c.inspectErr
	}
	return &wslc.ContainerInspect{Name: "/normalized", Config: wslc.ContainerConfig{Image: "nginx:latest"}, State: wslc.ContainerState{Status: "running"}}, nil
}

func TestCreateState(t *testing.T) {
	for _, failure := range []string{"", "start", "inspect"} {
		t.Run(failure, func(t *testing.T) {
			p := testPlan(t, map[string]tftypes.Value{"env": tftypes.NewValue(tftypes.Map{ElementType: tftypes.String}, map[string]tftypes.Value{"Z": tftypes.NewValue(tftypes.String, "last"), "A": tftypes.NewValue(tftypes.String, "first")})})
			c := &fakeClient{}
			if failure == "start" {
				c.startErr = errors.New("start failed")
			}
			if failure == "inspect" {
				c.inspectErr = errors.New("inspect failed")
			}
			resp := resource.CreateResponse{State: tfsdk.State{Schema: p.Schema, Raw: tftypes.NewValue(p.Raw.Type(), nil)}}
			(&containerResource{client: c}).Create(context.Background(), resource.CreateRequest{Plan: p}, &resp)
			if resp.Diagnostics.HasError() != (failure != "") {
				t.Fatal(resp.Diagnostics)
			}
			if !resp.State.Raw.IsFullyKnown() {
				t.Fatal("unknown in returned state")
			}
			var got containerResourceModel
			if d := resp.State.Get(context.Background(), &got); d.HasError() {
				t.Fatal(d)
			}
			if got.ID.ValueString() != "id123" || got.Image.ValueString() != "nginx" || got.Name.ValueString() != "test" {
				t.Fatalf("planned inputs or ID changed: %+v", got)
			}
			if failure != "" && !got.State.IsNull() {
				t.Fatal("failed observation must be null")
			}
			if failure == "" && got.State.ValueString() != "running" {
				t.Fatal("missing observed status")
			}
			if !reflect.DeepEqual(c.opts.Env, []string{"A=first", "Z=last"}) {
				t.Fatalf("env ordering %v", c.opts.Env)
			}
		})
	}
}

func TestReadImportAndMissing(t *testing.T) {
	for _, missing := range []bool{false, true} {
		p := testPlan(t, map[string]tftypes.Value{"id": tftypes.NewValue(tftypes.String, "id123"), "name": tftypes.NewValue(tftypes.String, nil), "image": tftypes.NewValue(tftypes.String, nil), "state": tftypes.NewValue(tftypes.String, nil)})
		c := &fakeClient{}
		if missing {
			c.inspectErr = wslc.ErrNotFound
		}
		state := tfsdk.State{Schema: p.Schema, Raw: p.Raw}
		resp := resource.ReadResponse{State: state}
		(&containerResource{client: c}).Read(context.Background(), resource.ReadRequest{State: state}, &resp)
		if resp.Diagnostics.HasError() {
			t.Fatal(resp.Diagnostics)
		}
		if missing {
			if !resp.State.Raw.IsNull() {
				t.Fatal("missing resource retained")
			}
			continue
		}
		var got containerResourceModel
		if d := resp.State.Get(context.Background(), &got); d.HasError() {
			t.Fatal(d)
		}
		if got.Name.ValueString() != "normalized" || got.Image.ValueString() != "nginx:latest" {
			t.Fatal("import attributes not populated")
		}
	}
}

func TestPortSchemaAndFormatting(t *testing.T) {
	var sr resource.SchemaResponse
	(&containerResource{}).Schema(context.Background(), resource.SchemaRequest{}, &sr)
	if !sr.Schema.Attributes["env"].(schema.MapAttribute).Sensitive {
		t.Fatal("env must be sensitive")
	}
	ports := sr.Schema.Attributes["ports"].(schema.ListNestedAttribute)
	protocol := ports.NestedObject.Attributes["protocol"].(schema.StringAttribute)
	var dr defaults.StringResponse
	protocol.Default.DefaultString(context.Background(), defaults.StringRequest{}, &dr)
	if dr.PlanValue.ValueString() != "tcp" {
		t.Fatal("missing tcp default")
	}
	for _, tc := range []struct {
		name    string
		n       int64
		invalid bool
	}{{"host_port", 0, false}, {"host_port", 65535, false}, {"host_port", -1, true}, {"host_port", 65536, true}, {"container_port", 0, true}, {"container_port", 80, false}} {
		var resp validator.Int64Response
		ports.NestedObject.Attributes[tc.name].(schema.Int64Attribute).Validators[0].ValidateInt64(context.Background(), validator.Int64Request{Path: path.Root("ports").AtListIndex(0).AtName(tc.name), ConfigValue: types.Int64Value(tc.n)}, &resp)
		if resp.Diagnostics.HasError() != tc.invalid {
			t.Fatalf("%+v: %v", tc, resp.Diagnostics)
		}
	}
	for _, p := range []string{"tcp", "udp", "", "sctp"} {
		var resp validator.StringResponse
		protocol.Validators[0].ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(p)}, &resp)
		if resp.Diagnostics.HasError() != (p == "" || p == "sctp") {
			t.Fatalf("%q: %v", p, resp.Diagnostics)
		}
	}
	got := buildPublishArgs([]containerPortModel{{HostIP: types.StringValue("127.0.0.1"), HostPort: types.Int64Value(8080), ContainerPort: types.Int64Value(80), Protocol: dr.PlanValue}})
	if !reflect.DeepEqual(got, []string{"127.0.0.1:8080:80/tcp"}) {
		t.Fatal(got)
	}
}
