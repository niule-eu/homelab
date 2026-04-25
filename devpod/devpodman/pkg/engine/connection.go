package engine

import "context"

// EngineConnection is an alias for context.Context.
// The podman bindings embed their HTTP client in a context
// returned by bindings.NewConnection(). This context must be
// passed to all podman API calls.
//
// Connection validation is handled by internal/podman.NewClient.
type EngineConnection = context.Context
