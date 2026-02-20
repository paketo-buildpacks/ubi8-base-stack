package utils

import (
	"bytes"
	"fmt"
	"os"

	"github.com/google/uuid"

	"github.com/paketo-buildpacks/occam"
	"github.com/paketo-buildpacks/packit/v2/pexec"
)

func GenerateBuilder(buildImage string, runImage string, registryUrl string) (buildImageUrl string, runImageUrl string, builderImageUrl string, err error) {

	buildImageID := fmt.Sprintf("build-image-%s", uuid.NewString())
	buildImageUrl, err = PushFileToLocalRegistry(buildImage, registryUrl, buildImageID)
	if err != nil {
		return "", "", "", err
	}

	runImageID := fmt.Sprintf("run-image-%s", uuid.NewString())
	runImageUrl, err = PushFileToLocalRegistry(runImage, registryUrl, runImageID)
	if err != nil {
		return "", "", "", err
	}

	// Creating builder file
	builderConfigFile, err := os.CreateTemp("", "builder.toml")
	if err != nil {
		return "", "", "", err
	}

	builderConfigFilepath := builderConfigFile.Name()

	_, err = fmt.Fprintf(builderConfigFile, `
			description= "UBI 8 base builder"
			[build]
			  image = "%s:latest"

			[run]
				[[run.images]]
					image = "%s:latest"

			[stack]
			  id = "io.buildpacks.stacks.ubi8"
			  build-image = "%s:latest"
			  run-image = "%s:latest"
			`, buildImageUrl, runImageUrl, buildImageUrl, runImageUrl)

	if err != nil {
		return "", "", "", err
	}

	// naming builder and pushing it to registry with pack cli
	builderImageUrl = fmt.Sprintf("%s/builder-%s", registryUrl, uuid.NewString())

	buf := bytes.NewBuffer(nil)

	pack := pexec.NewExecutable("pack")
	err = pack.Execute(pexec.Execution{
		Stdout: buf,
		Stderr: buf,
		Args: []string{
			"builder",
			"create",
			builderImageUrl,
			fmt.Sprintf("--config=%s", builderConfigFilepath),
			"--publish",
		},
	})

	if err != nil {
		return "", "", "", err
	}

	err = os.RemoveAll(builderConfigFilepath)
	if err != nil {
		return "", "", "", err
	}

	return buildImageUrl, runImageUrl, builderImageUrl, nil
}

func PushFileToLocalRegistry(filePath string, registryUrl string, imageName string) (string, error) {
	buf := bytes.NewBuffer(nil)
	imageURL := fmt.Sprintf("%s/%s", registryUrl, imageName)

	jam := pexec.NewExecutable("jam")
	err := jam.Execute(pexec.Execution{
		Stdout: buf,
		Stderr: buf,
		Args: []string{
			"publish-image",
			"--image-ref",
			imageURL,
			"--image-archive", filePath,
		},
	})

	if err != nil {
		return buf.String(), err
	} else {
		return imageURL, nil
	}
}

func RemoveImages(docker occam.Docker, imageIDs []string) error {

	for _, imageID := range imageIDs {
		err := docker.Image.Remove.Execute(imageID)
		if err != nil {
			return err
		}
	}

	return nil
}
