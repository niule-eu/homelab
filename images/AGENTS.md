# Vibe CLI Container Builds

**Use `podman-remote` instead of `podman`**

```bash
# Build
/usr/local/bin/podman-remote build -t IMAGE_NAME -f CONTAINERFILE .

# Run  
/usr/local/bin/podman-remote run --rm IMAGE_NAME

# Example
/usr/local/bin/podman-remote build -t hlcli -f images/hlcli/Containerfile.distroless .
/usr/local/bin/podman-remote run --rm hlcli --help
```

**Why?** The Vibe CLI execution context uses podman-remote to communicate with the host's podman daemon.