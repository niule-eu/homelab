package devcontainer

import (
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/distribution/reference"

	"github.com/niule-eu/devpodman/model"
)

// ResolvedConfig holds the parsed devcontainer configuration after
// CUE validation and Go priority resolution.
type ResolvedConfig struct {
	Build      *model.DockerfileContainer
	Image      *model.ImageContainer
	Common     *model.DevContainerCommon
	NonCompose *model.NonComposeBase
}

// Load parses and validates a devcontainer.json byte slice.
// It validates the JSON against individual CUE definitions and resolves
// conflicts via Go priority (dockerfile over image).
func Load(data []byte) (*ResolvedConfig, error) {
	ctx := cuecontext.New()
	schema := ctx.CompileString(model.Schema)
	if err := schema.Err(); err != nil {
		return nil, fmt.Errorf("failed to compile CUE schema: %w", err)
	}

	var (
		dockerfileDC model.DockerfileContainer
		imageDC      model.ImageContainer
		commonDC     model.DevContainerCommon
		nonComposeDC model.NonComposeBase
		errs         []string
	)

	// Parse JSON into each struct type independently (open — ignores extra fields)
	json.Unmarshal(data, &dockerfileDC)
	json.Unmarshal(data, &imageDC)
	json.Unmarshal(data, &commonDC)
	json.Unmarshal(data, &nonComposeDC)

	// Validate dockerfileContainer against its CUE definition
	dockerfileOK := validateStruct(ctx, schema, "#dockerfileContainer", dockerfileDC)
	if !dockerfileOK {
		errs = append(errs, "dockerfile: does not match #dockerfileContainer schema")
	}

	// Validate imageContainer against its CUE definition
	imageOK := validateStruct(ctx, schema, "#imageContainer", imageDC)

	// Neither matched
	if !dockerfileOK && !imageOK {
		detail := ""
		if len(errs) > 0 {
			detail = ": " + joinErrs(errs)
		}
		return nil, fmt.Errorf("devcontainer.json must specify either 'image' or 'build'%s", detail)
	}

	cfg := &ResolvedConfig{}

	// Best-effort: populate if schema passes
	if cOK := validateStruct(ctx, schema, "#devContainerCommon", commonDC); cOK {
		cfg.Common = &commonDC
	}
	if ncOK := validateStruct(ctx, schema, "#nonComposeBase", nonComposeDC); ncOK {
		cfg.NonCompose = &nonComposeDC
	}

	// Priority: dockerfile over image
	if dockerfileOK {
		cfg.Build = &dockerfileDC
		return cfg, nil
	}

	if err := validateImageReference(imageDC.Image); err != nil {
		return nil, err
	}
	cfg.Image = &imageDC
	return cfg, nil
}

// validateStruct encodes a Go struct as a CUE value and checks it against a definition.
func validateStruct[T any](ctx *cue.Context, schema cue.Value, defPath string, val T) bool {
	def := schema.LookupPath(cue.ParsePath(defPath))
	if !def.Exists() {
		return false
	}
	v := ctx.Encode(val)
	u := def.Unify(v)
	return u.Err() == nil
}

func validateImageReference(ref string) error {
	if ref == "" {
		return fmt.Errorf("'image' must not be empty")
	}
	if _, err := reference.ParseNormalizedNamed(ref); err != nil {
		return fmt.Errorf("'image' must be a valid container image reference, got %q", ref)
	}
	return nil
}

func joinErrs(errs []string) string {
	s := ""
	for i, e := range errs {
		if i > 0 {
			s += "; "
		}
		s += e
	}
	return s
}
