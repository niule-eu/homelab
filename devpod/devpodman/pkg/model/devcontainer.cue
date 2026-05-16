package model

// Defines a dev container
@jsonschema(schema="http://json-schema.org/draft-07/schema#")

// Mount definition for devpodman
#Mount: {
	source!: string
	target!: string
	type!:   "volume" | "bind"
}

// Build configuration options
#buildOptions: {
	// Target stage in a multi-stage build.
	target?: string

	// Build arguments.
	args?: [string]: string

	// The image to consider as a cache. Use an array to specify
	// multiple images.
	cacheFrom?: [...string]

	// The location of the Dockerfile that defines the contents of the
	// container. The path is relative to the folder containing the
	// `devcontainer.json` file.
	dockerfile!: string & =~"^[^/]"

	// The location of the context folder for building the Docker
	// image. The path is relative to the folder containing the
	// `devcontainer.json` file.
	context?: string & =~"^[^/]"
}

#devContainerCommon: {
	// A name for the dev container.
	name?: string & =~"^[a-z0-9A-Z][a-z0-9A-Z_.-]*[a-z0-9A-Z]$"

	// Features to add to the dev container.
	features?: {
		...
	}

	// Array consisting of the Feature id (without the semantic
	// version) of Features in the order the user wants them to be
	// installed.
	overrideFeatureInstallOrder?: [...string]

	// Ports that are forwarded from the container to the local
	// machine. Can be an integer port number, or a string of the
	// format "host:port_number".
	forwardPorts?: [...matchN(1, [int & <=65535 & >=0, =~"^([a-z0-9-]+):(\\d{1,5})$"])]
	portsAttributes?: close({
		{[=~"(^\\d+(-\\d+)?$)|(.+)"]: {
			// Defines the action that occurs when the port is discovered for
			// automatic forwarding
			onAutoForward?: "notify" | "openBrowser" | "openBrowserOnce" | "openPreview" | "silent" | "ignore"

			// Automatically prompt for elevation (if needed) when this port
			// is forwarded. Elevate is required if the local port is a
			// privileged port.
			elevateIfNeeded?: bool

			// Label that will be shown in the UI for this port.
			label?: string, requireLocalPort?: bool

			// The protocol to use when forwarding this port.
			protocol?: "http" | "https"
		}}
	})
	otherPortsAttributes?: close({
		// Defines the action that occurs when the port is discovered for
		// automatic forwarding
		onAutoForward?: "notify" | "openBrowser" | "openPreview" | "silent" | "ignore"

		// Automatically prompt for elevation (if needed) when this port
		// is forwarded. Elevate is required if the local port is a
		// privileged port.
		elevateIfNeeded?: bool

		// Label that will be shown in the UI for this port.
		label?:            string
		requireLocalPort?: bool

		// The protocol to use when forwarding this port.
		protocol?: "http" | "https"
	})

	// Controls whether on Linux the container's user should be
	// updated with the local user's UID and GID. On by default when
	// opening from a local folder.
	updateRemoteUserUID?: bool

	// Remote environment variables to set for processes spawned in
	// the container including lifecycle scripts and any remote
	// editor/IDE server process.
	remoteEnv?: [string]: string

	// The username to use for spawning processes in the container
	// including lifecycle scripts and any remote editor/IDE server
	// process. The default is the same user as the container.
	remoteUser?: string

	// A command string or list of command arguments to run on the
	// host machine during initialization, including during container
	// creation and on subsequent starts. The command may run more
	// than once during a given session. This command is run before
	// "onCreateCommand". If this is a single string, it will be run
	// in a shell. If this is an array of strings, it will be run as
	// a single command without shell.
	initializeCommand?: [...string]

	// A command to run when creating the container. This command is
	// run after "initializeCommand" and before
	// "updateContentCommand". If this is a single string, it will be
	// run in a shell. If this is an array of strings, it will be run
	// as a single command without shell.
	onCreateCommand?: {
		[string]: [...string]
	}

	// A command to run when creating the container and rerun when the
	// workspace content was updated while creating the container.
	// This command is run after "onCreateCommand" and before
	// "postCreateCommand". If this is a single string, it will be
	// run in a shell. If this is an array of strings, it will be run
	// as a single command without shell.
	updateContentCommand?: {
		[string]: [...string]
	}

	// A command to run after creating the container. This command is
	// run after "updateContentCommand" and before
	// "postStartCommand". If this is a single string, it will be run
	// in a shell. If this is an array of strings, it will be run as
	// a single command without shell.
	postCreateCommand?: {
		[string]: [...string]
	}

	// A command to run after starting the container. This command is
	// run after "postCreateCommand" and before "postAttachCommand".
	// If this is a single string, it will be run in a shell. If this
	// is an array of strings, it will be run as a single command
	// without shell.
	postStartCommand?: {
		[string]: [...string]
	}

	// A command to run when attaching to the container. This command
	// is run after "postStartCommand". If this is a single string,
	// it will be run in a shell. If this is an array of strings, it
	// will be run as a single command without shell.
	postAttachCommand?: {
		[string]: [...string]
	}

	// The user command to wait for before continuing execution in the
	// background while the UI is starting up. The default is
	// "updateContentCommand".
	waitFor?: "initializeCommand" | "onCreateCommand" | "updateContentCommand" | "postCreateCommand" | "postStartCommand"

	// User environment probe to run. The default is
	// "loginInteractiveShell".
	userEnvProbe?: "none" | "loginShell" | "loginInteractiveShell" | "interactiveShell"

	// Host hardware requirements.
	hostRequirements?: {
		// Number of required CPUs.
		cpus?: int & >=1

		// Amount of required RAM in bytes. Supports units tb, gb, mb and
		// kb.
		memory?: =~"^\\d+([tgmk]b)?$"

		// Amount of required disk space in bytes. Supports units tb, gb,
		// mb and kb.
		storage?: =~"^\\d+([tgmk]b)?$"
		gpu?: matchN(1, [true | false | "optional", close({
			// Number of required cores.
			cores?: int & >=1

			// Amount of required RAM in bytes. Supports units tb, gb, mb and
			// kb.
			memory?: =~"^\\d+([tgmk]b)?$"
		})])
	}

	// Tool-specific configuration. Each tool should use a JSON object
	// subproperty with a unique name to group its customizations.
	customizations?: {
		devpodman?: #devpodmanCustomization
	}
	additionalProperties?: {
		...
	}
}

#dockerfileContainer: {
	// Docker build-related options.
	build!: #buildOptions
}

#imageContainer: {
	// The docker image that will be used to create the container.
	image!: string & =~"^.+$"
}

#nonComposeBase: {
	// Application ports that are exposed by the container. Is passed to Docker 
	// unchanged. Can be used to map ports differently, e.g. "8000:8010".
	appPort?: [...string]

	// Container environment variables.
	containerEnv?: [string]: string

	// The user the container will be started with. The default is the
	// user on the Docker image.
	containerUser?: string

	// Mount points to set up when creating the container. 
	mounts?: [...#Mount]

	// The arguments required when starting in the container.
	runArgs?: [...string]

	// Action to take when the user disconnects from the container in
	// their editor. The default is to stop the container.
	shutdownAction?: "none" | "stopContainer"

	privileged?: bool
	// Whether to overwrite the command specified in the image. The
	// default is true.
	overrideCommand?: bool

	// The path of the workspace folder inside the container.
	workspaceFolder?: string

	// The --mount parameter for docker run. The default is to mount
	// the project folder at /workspaces/$project.
	workspaceMount?: #Mount
}

#devpodmanCustomization: {
	command:    [...string] // defaults to *["sleep", "infinity"]
	args:       [...string] // defaults to *[]
	workdir:    #devpodmanWorkdir
	network:    #devpodmanNetwork
	codeServer: #devpodmanCodeServer
}

#devpodmanNetwork: {
	enabled: bool | *true
	name?:   string & =~"^[a-z0-9A-Z][a-z0-9A-Z_.-]*[a-z0-9A-Z]$" // defaults to devpod name + '-network'
}

#devpodmanCodeServer: {
	enabled:       bool | *true
	containerPort: int & <=65535 & >=0 | *8080
	hostPort:      int & <=65535 & >=0 | *8080
}

#devpodmanWorkdir: {
	emptyVol: bool | *false
}
