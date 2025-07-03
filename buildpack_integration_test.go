package acceptance_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	utils "github.com/paketo-buildpacks/ubi8-base-stack/internal/utils"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"

	"github.com/paketo-buildpacks/occam"
	. "github.com/paketo-buildpacks/occam/matchers"
)

func testBuildpackIntegration(t *testing.T, context spec.G, it spec.S) {
	var (
		Expect     = NewWithT(t).Expect
		Eventually = NewWithT(t).Eventually

		err error

		pack   occam.Pack
		docker occam.Docker
		source string
		name   string

		image     occam.Image
		container occam.Container

		runImageUrl     string
		builderImageUrl string
	)

	it.Before(func() {
		pack = occam.NewPack().WithVerbose()
		docker = occam.NewDocker()
	})

	context("When building GO app using default stack", func() {

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			err = utils.RemoveImages(docker, []string{runImageUrl, builderImageUrl})
			Expect(err).NotTo(HaveOccurred())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it.Before(func() {
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())

			source, err = occam.Source(filepath.Join("integration", "testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())
		})

		for _, stack := range settings.ImagesJson.StackImages {
			// Create a copy of the stack to get the value instead of a pointer
			stack := stack

			if stack.Name != "default" {
				continue
			}

			it("should successfully build a go app", func() {
				_, runImageUrl, builderImageUrl, err = utils.GenerateBuilder(
					filepath.Join(root, DefaultStack.OutputDir, "build.oci"),
					filepath.Join(root, stack.OutputDir, "run.oci"),
					RegistryUrl,
				)
				Expect(err).NotTo(HaveOccurred())

				image, _, err = pack.WithNoColor().Build.
					WithBuildpacks(
						settings.Buildpacks.GoDist.Online,
						settings.Buildpacks.BuildPlan.Online,
					).
					WithEnv(map[string]string{
						"BP_LOG_LEVEL": "DEBUG",
					}).
					WithPullPolicy("if-not-present").
					WithBuilder(builderImageUrl).
					Execute(name, source)
				Expect(err).NotTo(HaveOccurred())

				container, err = docker.Container.Run.
					WithDirect().
					WithCommand("go").
					WithCommandArgs([]string{"run", "main.go"}).
					WithEnv(map[string]string{"PORT": "8080"}).
					WithPublish("8080").
					WithPublishAll().
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())
				Eventually(container).Should(Serve(MatchRegexp(`go1.*`)).OnPort(8080))

			})
		}
	})

	context("When building a GO app using Node.js stacks", func() {

		it.After(func() {
			Expect(docker.Container.Remove.Execute(container.ID)).To(Succeed())
			Expect(docker.Image.Remove.Execute(image.ID)).To(Succeed())
			Expect(docker.Volume.Remove.Execute(occam.CacheVolumeNames(name))).To(Succeed())
			err = utils.RemoveImages(docker, []string{runImageUrl, builderImageUrl})
			Expect(err).NotTo(HaveOccurred())
			Expect(os.RemoveAll(source)).To(Succeed())
		})

		it.Before(func() {
			name, err = occam.RandomName()
			Expect(err).NotTo(HaveOccurred())

			source, err = occam.Source(filepath.Join("integration", "testdata", "simple_app"))
			Expect(err).NotTo(HaveOccurred())
		})

		nodejsRegex, _ := regexp.Compile("^nodejs")

		for _, stack := range settings.ImagesJson.StackImages {
			// Create a copy of the stack to get the value instead of a pointer
			stack := stack

			if !nodejsRegex.MatchString(stack.Name) {
				continue
			}

			it(fmt.Sprintf("it should successfully get the %s version of the run image", stack.Name), func() {
				_, runImageUrl, builderImageUrl, err = utils.GenerateBuilder(
					filepath.Join(root, DefaultStack.OutputDir, "build.oci"),
					filepath.Join(root, stack.OutputDir, "run.oci"),
					RegistryUrl,
				)
				Expect(err).NotTo(HaveOccurred())

				image, _, err = pack.WithNoColor().Build.
					WithBuildpacks(
						settings.Buildpacks.GoDist.Online,
						settings.Buildpacks.BuildPlan.Online,
					).
					WithEnv(map[string]string{
						"BP_LOG_LEVEL": "DEBUG",
					}).
					WithPullPolicy("if-not-present").
					WithBuilder(builderImageUrl).
					Execute(name, source)
				Expect(err).NotTo(HaveOccurred())

				container, err = docker.Container.Run.
					WithDirect().
					WithCommand("go").
					WithCommandArgs([]string{"run", "main.go"}).
					WithEnv(map[string]string{"PORT": "8080"}).
					WithPublish("8080").
					WithPublishAll().
					Execute(image.ID)
				Expect(err).NotTo(HaveOccurred())

				Eventually(container).Should(BeAvailable())
				Eventually(container).Should(Serve(MatchRegexp(`go1.*`)).OnPort(8080))
				nodejsMajorVersion := stack.Name[len("nodejs-"):]
				Eventually(container).Should(Serve(MatchRegexp(fmt.Sprintf(`v%s.*`, nodejsMajorVersion))).OnPort(8080).WithEndpoint("/nodejs/version"))
			})
		}
	})
}
