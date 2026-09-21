package wslc

// ContainerInspect is the shape wslc.exe prints for
// `wslc inspect <id> --type container --format json` (one element of the
// top-level JSON array). It mirrors the Docker Engine API's container
// inspect response, which wslc.exe's output is compatible with.
type ContainerInspect struct {
	Id      string
	Name    string
	Image   string
	Created string
	Config  ContainerConfig
	State   ContainerState
	// Ports maps "<containerPort>/<proto>" (e.g. "80/tcp") to the host
	// bindings published for it, if any.
	Ports map[string][]PortBinding
}

type ContainerConfig struct {
	Cmd        []string
	Entrypoint []string
	Env        []string
	// Image is the image reference exactly as given at creation (e.g.
	// "nginx:latest"). Unlike the top-level ContainerInspect.Image (the
	// resolved "sha256:..." digest), this is the human-readable reference
	// this provider's "image" attribute expects to read back.
	Image       string
	Labels      map[string]string
	User        string
	WorkingDir  string
	Domainname  string
	Hostname    string
	StopSignal  string
	StopTimeout *int
}

type ContainerState struct {
	Running    bool
	Status     string
	ExitCode   int
	StartedAt  string
	FinishedAt string
}

type PortBinding struct {
	HostIp   string
	HostPort string
}

// CreateContainerOptions describes a new container to create via
// `wslc create`. Field order follows `wslc create --help`: Image and
// Command first (its Arguments, in their listed order), then every Option
// in the exact order that help text lists them.
type CreateContainerOptions struct {
	// Image is the image to create the container from, e.g.
	// "nginx:latest". Required.
	Image string

	// Command overrides the image's default command, if set.
	Command []string

	// CPUs is the CPU limit passed as `--cpus`, e.g. "0.5", "1", "2.5".
	CPUs string

	// DNS holds nameserver IPs passed as repeated `--dns` flags.
	DNS []string

	// DNSOptions holds resolv.conf options passed as repeated
	// `--dns-option` flags.
	DNSOptions []string

	// DNSSearch holds DNS search domains passed as repeated
	// `--dns-search` flags.
	DNSSearch []string

	// Domainname is the container's domain name, passed as `--domainname`.
	Domainname string

	// Entrypoint overrides the image's init process executable, passed
	// as `--entrypoint`.
	Entrypoint string

	// Env holds "KEY=VALUE" pairs passed as repeated `--env` flags.
	Env []string

	// Hostname is the container's host name, passed as `--hostname`.
	Hostname string

	// IP assigns the container a static IPv4 address on Network, passed
	// as `--ip`.
	IP string

	// Labels holds metadata passed as repeated `--label key=value` flags.
	Labels map[string]string

	// Memory is the memory limit passed as `--memory`, e.g. "512M", "1G".
	Memory string

	// Name is the container's name. Optional: wslc.exe assigns a random
	// name when omitted, but this provider always sets it explicitly so
	// the resulting container has a predictable, Terraform-chosen identity.
	Name string

	// Network connects the container to the named network, passed as
	// `--network`.
	Network string

	// Publish holds host/container port mappings passed as repeated
	// `--publish` flags, each formatted like
	// "[hostIP:]hostPort:containerPort[/protocol]".
	Publish []string

	// PublishAll publishes every exposed port to a random host port,
	// passed as `--publish-all`.
	PublishAll bool

	// PullPolicy controls whether `wslc create` pulls Image before
	// creating the container: "always", "missing", or "never", passed as
	// `--pull`. Empty means "let wslc.exe use its own default", which is
	// "missing" (pull only if the image is not already present locally).
	PullPolicy string

	// Remove removes the container once it stops, passed as `--rm`.
	Remove bool

	// ShmSize is the size of /dev/shm passed as `--shm-size`, e.g. "64M".
	ShmSize string

	// StopSignal is the signal `wslc stop` sends by default, passed as
	// `--stop-signal`.
	StopSignal string

	// StopTimeout is the number of seconds `wslc stop` waits by default
	// before killing the container (-1 for no timeout), passed as
	// `--stop-timeout`. nil means "let wslc.exe use its own default".
	StopTimeout *int

	// Tmpfs holds tmpfs mount specs (e.g. "/mytmp" or
	// "/mytmp:size=100m") passed as repeated `--tmpfs` flags.
	Tmpfs []string

	// Ulimits holds ulimit specs ("<name>=<soft>[:<hard>]", -1 for
	// unlimited) passed as repeated `--ulimit` flags.
	Ulimits []string

	// User selects the user (name|uid|uid:gid) the process runs as,
	// passed as `--user`.
	User string

	// WorkDir is the working directory inside the container, passed as
	// `--workdir`.
	WorkDir string
}
