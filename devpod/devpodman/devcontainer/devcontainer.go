package devcontainer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/distribution/reference"
	"github.com/niule-eu/devpodman/model/build"
	"github.com/niule-eu/devpodman/model/common"
	"github.com/niule-eu/devpodman/model/image"
)

type containerDiscriminator struct {
	Image string            `json:"image,omitempty"`
	Build *build.BuildProps `json:"build,omitempty"`
}

func Load(data []byte) (*build.BuildProperties, *image.ImageProperties, *common.CommonProperties, error) {
	var discriminator containerDiscriminator
	if err := json.Unmarshal(data, &discriminator); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse devcontainer.json: %w", err)
	}

	var commonProps common.CommonProperties
	if err := json.Unmarshal(data, &commonProps); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse devcontainer.json: %w", err)
	}

	if discriminator.Image != "" {
		if discriminator.Build != nil {
			return nil, nil, nil, fmt.Errorf("devcontainer.json must specify either 'image' or 'build', not both")
		}
		imgProps := &image.ImageProperties{Image: discriminator.Image}
		if err := validateImage(imgProps); err != nil {
			return nil, nil, nil, err
		}
		if err := validateCommon(&commonProps); err != nil {
			return nil, nil, nil, err
		}
		return nil, imgProps, &commonProps, nil
	}

	if discriminator.Build != nil {
		buildProps := &build.BuildProperties{Build: *discriminator.Build}
		if err := validateBuild(buildProps); err != nil {
			return nil, nil, nil, err
		}
		if err := validateCommon(&commonProps); err != nil {
			return nil, nil, nil, err
		}
		return buildProps, nil, &commonProps, nil
	}

	return nil, nil, nil, fmt.Errorf("devcontainer.json must specify either 'image' or 'build'")
}

func validateImage(img *image.ImageProperties) error {
	if img.Image == "" {
		return fmt.Errorf("'image' must not be empty")
	}
	if err := validateImageReference(img.Image); err != nil {
		return err
	}
	return nil
}

func validateBuild(bld *build.BuildProperties) error {
	if bld.Build.Dockerfile == "" {
		return fmt.Errorf("'build.dockerfile' must not be empty")
	}
	if filepath.IsAbs(bld.Build.Dockerfile) {
		return fmt.Errorf("'build.dockerfile' must be a relative path, got %q", bld.Build.Dockerfile)
	}
	if bld.Build.Context != nil && filepath.IsAbs(*bld.Build.Context) {
		return fmt.Errorf("'build.context' must be a relative path, got %q", *bld.Build.Context)
	}
	return nil
}

func validateCommon(c *common.CommonProperties) error {
	if c.WorkspaceMount != nil && c.WorkspaceFolder == nil {
		return fmt.Errorf("'workspaceFolder' must be set when 'workspaceMount' is specified")
	}
	if c.WorkspaceFolder != nil && c.WorkspaceMount == nil {
		return fmt.Errorf("'workspaceMount' must be set when 'workspaceFolder' is specified")
	}
	if c.WorkspaceFolder != nil && !filepath.IsAbs(*c.WorkspaceFolder) {
		return fmt.Errorf("'workspaceFolder' must be an absolute path, got %q", *c.WorkspaceFolder)
	}
	return nil
}

func validateImageReference(ref string) error {
	if _, err := reference.ParseNormalizedNamed(ref); err != nil {
		return fmt.Errorf("'image' must be a valid container image reference, got %q", ref)
	}
	return nil
}
