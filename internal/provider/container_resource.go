package provider

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/jmnote/terraform-provider-wslc/internal/wslc"
)

var (
	_ resource.Resource                   = &containerResource{}
	_ resource.ResourceWithConfigure      = &containerResource{}
	_ resource.ResourceWithImportState    = &containerResource{}
	_ resource.ResourceWithValidateConfig = &containerResource{}
)

// validPullPolicies mirrors the values `wslc create --pull` accepts.
var validPullPolicies = map[string]bool{"always": true, "missing": true, "never": true}

func NewContainerResource() resource.Resource {
	return &containerResource{}
}

type containerResource struct {
	client wslc.Client
}

// containerPortModel is one element of the `ports` nested list.
type containerPortModel struct {
	HostIP        types.String `tfsdk:"host_ip"`
	HostPort      types.Int64  `tfsdk:"host_port"`
	ContainerPort types.Int64  `tfsdk:"container_port"`
	Protocol      types.String `tfsdk:"protocol"`
}

// containerResourceModel mirrors the wslc_container schema. Field order
// follows `wslc create --help`: id first (our own identity, not a wslc
// concept), then image and command (its Arguments, in their listed order),
// then every Option in the exact order that help text lists them, and
// finally state (our own computed status, likewise not a wslc create
// concept). Every attribute besides id and state is immutable after
// creation: wslc.exe has no in-place update for a container's image,
// command, environment, labels, or published ports, so changing any of
// them replaces the resource.
type containerResourceModel struct {
	ID          types.String         `tfsdk:"id"`
	Image       types.String         `tfsdk:"image"`
	Command     []types.String       `tfsdk:"command"`
	CPUs        types.String         `tfsdk:"cpus"`
	DNS         []types.String       `tfsdk:"dns"`
	DNSOptions  []types.String       `tfsdk:"dns_options"`
	DNSSearch   []types.String       `tfsdk:"dns_search"`
	Domainname  types.String         `tfsdk:"domainname"`
	Entrypoint  types.String         `tfsdk:"entrypoint"`
	Env         map[string]string    `tfsdk:"env"`
	Hostname    types.String         `tfsdk:"hostname"`
	IP          types.String         `tfsdk:"ip"`
	Labels      map[string]string    `tfsdk:"labels"`
	Memory      types.String         `tfsdk:"memory"`
	Name        types.String         `tfsdk:"name"`
	Network     types.String         `tfsdk:"network"`
	Ports       []containerPortModel `tfsdk:"ports"`
	PublishAll  types.Bool           `tfsdk:"publish_all"`
	Pull        types.String         `tfsdk:"pull"`
	Remove      types.Bool           `tfsdk:"rm"`
	ShmSize     types.String         `tfsdk:"shm_size"`
	StopSignal  types.String         `tfsdk:"stop_signal"`
	StopTimeout types.Int64          `tfsdk:"stop_timeout"`
	Tmpfs       []types.String       `tfsdk:"tmpfs"`
	Ulimits     []types.String       `tfsdk:"ulimit"`
	User        types.String         `tfsdk:"user"`
	WorkDir     types.String         `tfsdk:"workdir"`
	State       types.String         `tfsdk:"state"`
}

func (r *containerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_container"
}

func (r *containerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a WSL container's lifecycle via wslc.exe: `wslc create` followed by " +
			"`wslc start`, and `wslc remove --force` on destroy. Every attribute besides `id` and `state` " +
			"is immutable: wslc.exe has no in-place update for a container's image, command, environment, " +
			"labels, or published ports, so changing any of them replaces the container.",
		// Attributes are ordered to match `wslc create --help`: id first (our
		// own identity), then image and command (its Arguments), then every
		// Option in the exact order that help text lists them, then state
		// (our own computed status) last.
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "The container's full ID, as assigned by wslc.exe. Also the `terraform import` identity.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"image": schema.StringAttribute{
				Required:    true,
				Description: "The image to create the container from, e.g. \"nginx:latest\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"command": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Overrides the image's default command.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"cpus": schema.StringAttribute{
				Optional:    true,
				Description: "CPU limit, e.g. \"0.5\", \"1\", \"2.5\", passed as `wslc create --cpus`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dns": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "DNS nameserver IPs written into the container's resolv.conf, passed as repeated `wslc create --dns`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"dns_options": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "resolv.conf options (e.g. \"ndots:5\"), passed as repeated `wslc create --dns-option`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"dns_search": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "DNS search domains, passed as repeated `wslc create --dns-search`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"domainname": schema.StringAttribute{
				Optional:    true,
				Description: "The container's domain name, passed as `wslc create --domainname`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"entrypoint": schema.StringAttribute{
				Optional:    true,
				Description: "Overrides the image's init process executable, passed as `wslc create --entrypoint`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"env": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Environment variables set in the container, passed as repeated `wslc create --env`.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"hostname": schema.StringAttribute{
				Optional:    true,
				Description: "The container's host name, passed as `wslc create --hostname`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ip": schema.StringAttribute{
				Optional:    true,
				Description: "Static IPv4 address to assign the container on `network`, passed as `wslc create --ip`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Metadata set on the container, passed as repeated `wslc create --label`.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"memory": schema.StringAttribute{
				Optional:    true,
				Description: "Memory limit, e.g. \"512M\", \"1G\", passed as `wslc create --memory`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The container's name, passed as `wslc create --name`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"network": schema.StringAttribute{
				Optional:    true,
				Description: "Name of the network to connect the container to, passed as `wslc create --network`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ports": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Ports to publish from the container to the host, passed as repeated `wslc create --publish`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"host_ip": schema.StringAttribute{
							Optional:    true,
							Description: "Host IP address to bind the published port to. Defaults to all interfaces.",
						},
						"host_port": schema.Int64Attribute{
							Required:    true,
							Description: "Port on the host to publish to.",
						},
						"container_port": schema.Int64Attribute{
							Required:    true,
							Description: "Port inside the container to publish.",
						},
						"protocol": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Port protocol: \"tcp\" or \"udp\". Defaults to \"tcp\".",
						},
					},
				},
			},
			"publish_all": schema.BoolAttribute{
				Optional:    true,
				Description: "Publishes every exposed port to a random host port, passed as `wslc create --publish-all`.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"pull": schema.StringAttribute{
				Optional: true,
				Description: "Image pull policy applied during creation: \"always\", \"missing\", or \"never\", " +
					"passed as `wslc create --pull`. Defaults to wslc.exe's own default, \"missing\" (pull " +
					"only if the image isn't already present locally).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"rm": schema.BoolAttribute{
				Optional:    true,
				Description: "Removes the container once it stops, passed as `wslc create --rm`.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"shm_size": schema.StringAttribute{
				Optional:    true,
				Description: "Size of /dev/shm, e.g. \"64M\", passed as `wslc create --shm-size`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"stop_signal": schema.StringAttribute{
				Optional:    true,
				Description: "Signal `wslc stop` sends by default, passed as `wslc create --stop-signal`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"stop_timeout": schema.Int64Attribute{
				Optional: true,
				Description: "Seconds `wslc stop` waits by default before killing the container (-1 for no " +
					"timeout), passed as `wslc create --stop-timeout`.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"tmpfs": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "tmpfs mounts (e.g. \"/mytmp\" or \"/mytmp:size=100m\"), passed as repeated `wslc create --tmpfs`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"ulimit": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Ulimit specs (format: \"<name>=<soft>[:<hard>]\", -1 for unlimited), passed as repeated `wslc create --ulimit`.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"user": schema.StringAttribute{
				Optional:    true,
				Description: "User the process runs as (name|uid|uid:gid), passed as `wslc create --user`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workdir": schema.StringAttribute{
				Optional:    true,
				Description: "Working directory inside the container, passed as `wslc create --workdir`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Computed:    true,
				Description: "The container's current run status (e.g. \"running\", \"exited\"), as last observed from `wslc inspect`.",
			},
		},
	}
}

func (r *containerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(wslc.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected resource configure type",
			fmt.Sprintf("Expected wslc.Client, got: %T. This is a bug in the provider.", req.ProviderData),
		)
		return
	}
	r.client = client
}

// ValidateConfig catches an invalid `pull` value at `terraform plan`/`validate`
// time rather than only surfacing it as an apply-time error from wslc.exe.
func (r *containerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config containerResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.Pull.IsNull() || config.Pull.IsUnknown() {
		return
	}
	if pull := config.Pull.ValueString(); pull != "" && !validPullPolicies[pull] {
		resp.Diagnostics.AddAttributeError(
			path.Root("pull"),
			"Invalid pull policy",
			fmt.Sprintf("pull must be one of \"always\", \"missing\", or \"never\"; got %q.", pull),
		)
	}
}

func (r *containerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan containerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// "protocol" is Optional+Computed with no schema-level default, so an
	// omitted entry arrives here Unknown; resolve it to "tcp" now, both
	// for the --publish value built below and because Terraform requires
	// every attribute to be known before this model is saved to state.
	for i := range plan.Ports {
		if plan.Ports[i].Protocol.IsNull() || plan.Ports[i].Protocol.IsUnknown() || plan.Ports[i].Protocol.ValueString() == "" {
			plan.Ports[i].Protocol = types.StringValue("tcp")
		}
	}

	// Fields are listed alphabetically by their `wslc create` flag name,
	// matching the schema attribute order above.
	opts := wslc.CreateContainerOptions{
		CPUs:       plan.CPUs.ValueString(),
		Domainname: plan.Domainname.ValueString(),
		Entrypoint: plan.Entrypoint.ValueString(),
		Hostname:   plan.Hostname.ValueString(),
		Image:      plan.Image.ValueString(),
		IP:         plan.IP.ValueString(),
		Labels:     plan.Labels,
		Memory:     plan.Memory.ValueString(),
		Name:       plan.Name.ValueString(),
		Network:    plan.Network.ValueString(),
		Publish:    buildPublishArgs(plan.Ports),
		PublishAll: plan.PublishAll.ValueBool(),
		PullPolicy: plan.Pull.ValueString(),
		Remove:     plan.Remove.ValueBool(),
		ShmSize:    plan.ShmSize.ValueString(),
		StopSignal: plan.StopSignal.ValueString(),
		User:       plan.User.ValueString(),
		WorkDir:    plan.WorkDir.ValueString(),
	}
	if !plan.StopTimeout.IsNull() && !plan.StopTimeout.IsUnknown() {
		timeout := int(plan.StopTimeout.ValueInt64())
		opts.StopTimeout = &timeout
	}
	for _, c := range plan.Command {
		opts.Command = append(opts.Command, c.ValueString())
	}
	for _, d := range plan.DNS {
		opts.DNS = append(opts.DNS, d.ValueString())
	}
	for _, d := range plan.DNSOptions {
		opts.DNSOptions = append(opts.DNSOptions, d.ValueString())
	}
	for _, d := range plan.DNSSearch {
		opts.DNSSearch = append(opts.DNSSearch, d.ValueString())
	}
	for k, v := range plan.Env {
		opts.Env = append(opts.Env, k+"="+v)
	}
	for _, t := range plan.Tmpfs {
		opts.Tmpfs = append(opts.Tmpfs, t.ValueString())
	}
	for _, u := range plan.Ulimits {
		opts.Ulimits = append(opts.Ulimits, u.ValueString())
	}

	id, err := r.client.CreateContainer(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error creating wslc container", err.Error())
		return
	}
	plan.ID = types.StringValue(id)

	if err := r.client.StartContainer(ctx, id); err != nil {
		// The container exists (and so must be tracked in state) even
		// though starting it failed; report the start error but still
		// save what was created rather than losing track of it.
		resp.Diagnostics.AddError("Error starting wslc container", err.Error())
		plan.State = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *containerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state containerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	inspect, err := r.client.InspectContainer(ctx, state.ID.ValueString())
	if errors.Is(err, wslc.ErrNotFound) {
		// Reconcile drift: something outside Terraform (e.g. `wslc
		// remove` run manually) removed this container. Removing it
		// from state causes the next plan to propose recreating it,
		// rather than silently drifting or erroring.
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading wslc container", err.Error())
		return
	}

	applyInspect(&state, inspect)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only runs when a change to a Computed-only attribute is proposed,
// since every configurable attribute in the schema forces replacement; it
// simply re-reads the container's current state.
func (r *containerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan containerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(r.refresh(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *containerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state containerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveContainer(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Error deleting wslc container", err.Error())
	}
}

func (r *containerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// refresh re-reads the container identified by model.ID and overwrites
// model's computed attributes (state) with the observed values.
func (r *containerResource) refresh(ctx context.Context, model *containerResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	inspect, err := r.client.InspectContainer(ctx, model.ID.ValueString())
	if err != nil {
		diags.AddError("Error reading wslc container after apply", err.Error())
		return diags
	}
	applyInspect(model, inspect)
	return diags
}

// applyInspect overwrites model's Required attributes (name, image) and its
// state attribute from inspect. name and image are included (not just
// state) because wslc.exe reliably reports both back and, unlike this
// resource's other Optional+RequiresReplace attributes (cpus, dns, etc.),
// Terraform requires every Required attribute to hold a known, non-null
// value in state -- including right after `terraform import`, when
// ImportState has populated nothing but id. Without this, an imported
// resource's first plan would always propose destroying and recreating it,
// since state would have name/image as null while config has them set.
func applyInspect(model *containerResourceModel, inspect *wslc.ContainerInspect) {
	model.Name = types.StringValue(strings.TrimPrefix(inspect.Name, "/"))
	model.Image = types.StringValue(inspect.Config.Image)
	model.State = types.StringValue(inspect.State.Status)
}

// buildPublishArgs renders each ports entry as a `wslc create --publish`
// value: "[hostIP:]hostPort:containerPort[/protocol]".
func buildPublishArgs(ports []containerPortModel) []string {
	args := make([]string, 0, len(ports))
	for _, p := range ports {
		protocol := p.Protocol.ValueString()
		if protocol == "" {
			protocol = "tcp"
		}
		spec := strconv.FormatInt(p.HostPort.ValueInt64(), 10) + ":" + strconv.FormatInt(p.ContainerPort.ValueInt64(), 10) + "/" + protocol
		if hostIP := p.HostIP.ValueString(); hostIP != "" {
			spec = hostIP + ":" + spec
		}
		args = append(args, spec)
	}
	return args
}
