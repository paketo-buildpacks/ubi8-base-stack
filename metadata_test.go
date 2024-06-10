package acceptance_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	"github.com/paketo-buildpacks/packit/v2/vacation"
	"github.com/sclevine/spec"

	. "github.com/paketo-buildpacks/occam/matchers"
)

type Images struct {
	SupportUsns       bool    `json:"support_usns"`
	UpdateOnNewImage  bool    `json:"update_on_new_image"`
	receiptsShowLimit int     `json:"receipts_show_limit"`
	Images            []Image `json:"images"`
}

type Image struct {
	Name                    string `json:"name"`
	configDir               string `json:"config_dir"`
	OutputDir               string `json:"output_dir"`
	BuildImage              string `json:"build_image"`
	RunImage                string `json:"run_image"`
	BuildReceiptFilename    string `json:"build_receipt_filename"`
	RunReceiptFilename      string `json:"run_receipt_filename"`
	CreateBuildImage        bool   `json:"create_build_image"`
	BaseBuildContainerImage string `json:"base_build_container_image"`
	BaseRunContainerImage   string `json:"base_run_container_image"`
}

func testMetadata(t *testing.T, context spec.G, it spec.S) {

	jsonFile, err := os.Open("images.json")
	if err != nil {
		fmt.Println(err)
	}

	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

	var images Images

	err = json.Unmarshal(byteValue, &images)
	if err != nil {
		fmt.Println(err)
	}

	var (
		Expect = NewWithT(t).Expect

		tmpDir string
	)

	it.Before(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "")
		Expect(err).NotTo(HaveOccurred())
	})

	it.After(func() {
		Expect(os.RemoveAll(tmpDir)).To(Succeed())
	})

	it("builds base stack", func() {
		var (
			buildReleaseDate time.Time
			runReleaseDate   time.Time
		)

		by("confirming that the build image is correct", func() {
			index, manifests, err := getImageIndexAndManifests(tmpDir, filepath.Join(root, "./build", "build.oci"))
			Expect(err).NotTo(HaveOccurred())

			Expect(manifests).To(HaveLen(1))
			Expect(manifests[0].Platform).To(Equal(&v1.Platform{
				OS:           "linux",
				Architecture: "amd64",
			}))

			image, err := index.Image(manifests[0].Digest)
			Expect(err).NotTo(HaveOccurred())

			file, err := image.ConfigFile()
			Expect(err).NotTo(HaveOccurred())

			Expect(file.Config.Labels).To(SatisfyAll(
				HaveKeyWithValue("io.buildpacks.stack.id", "io.buildpacks.stacks.ubi8"),
				HaveKeyWithValue("io.buildpacks.stack.description", "base build ubi8 image to support buildpacks"),
				HaveKeyWithValue("io.buildpacks.stack.distro.name", "rhel"),
				HaveKeyWithValue("io.buildpacks.stack.distro.version", MatchRegexp(`8\.\d+`)),
				HaveKeyWithValue("io.buildpacks.stack.homepage", "https://github.com/paketo-community/ubi-base-stack"),
				HaveKeyWithValue("io.buildpacks.stack.maintainer", "Paketo Community"),
				HaveKeyWithValue("io.buildpacks.stack.metadata", MatchJSON("{}")),
			))

			buildReleaseDate, err = time.Parse(time.RFC3339, file.Config.Labels["io.buildpacks.stack.released"])
			Expect(err).NotTo(HaveOccurred())
			Expect(buildReleaseDate).NotTo(BeZero())

			Expect(image).To(SatisfyAll(
				HaveFileWithContent("/etc/group", ContainSubstring("cnb:x:1000:")),
				HaveFileWithContent("/etc/passwd", ContainSubstring("cnb:x:1002:1000::/home/cnb:/bin/bash")),
				HaveDirectory("/home/cnb"),
			))

			Expect(file.Config.User).To(Equal("1002:1000"))

			Expect(file.Config.Env).To(ContainElements(
				"CNB_USER_ID=1002",
				"CNB_GROUP_ID=1000",
				"CNB_STACK_ID=io.buildpacks.stacks.ubi8",
			))
		})

		for _, imageInfo := range images.Images {
			by(fmt.Sprintf("confirming that the run %s image is correct", imageInfo.Name), func() {

				index, manifests, err := getImageIndexAndManifests(tmpDir, filepath.Join(root, fmt.Sprintf("./%s", imageInfo.OutputDir), "run.oci"))
				Expect(err).NotTo(HaveOccurred())

				Expect(manifests).To(HaveLen(1))
				Expect(manifests[0].Platform).To(Equal(&v1.Platform{
					OS:           "linux",
					Architecture: "amd64",
				}))

				image, err := index.Image(manifests[0].Digest)
				Expect(err).NotTo(HaveOccurred())

				file, err := image.ConfigFile()
				Expect(err).NotTo(HaveOccurred())

				Expect(file.Config.Labels).To(SatisfyAll(
					HaveKeyWithValue("io.buildpacks.stack.id", "io.buildpacks.stacks.ubi8"),
					HaveKeyWithValue("io.buildpacks.stack.distro.name", "rhel"),
					HaveKeyWithValue("io.buildpacks.stack.distro.version", MatchRegexp(`8\.\d+`)),
					HaveKeyWithValue("io.buildpacks.stack.homepage", "https://github.com/paketo-community/ubi-base-stack"),
					HaveKeyWithValue("io.buildpacks.stack.maintainer", "Paketo Community"),
					HaveKeyWithValue("io.buildpacks.stack.metadata", MatchJSON("{}")),
				))

				if imageInfo.Name == "default" {
					Expect(file.Config.Labels).To(SatisfyAll(
						HaveKeyWithValue("io.buildpacks.stack.description", fmt.Sprintf("base %s ubi8 image to support buildpacks", imageInfo.RunImage)),
					))

				} else {
					Expect(file.Config.Labels).To(SatisfyAll(
						HaveKeyWithValue("io.buildpacks.stack.description", fmt.Sprintf("ubi8 %s image to support buildpacks", imageInfo.Name)),
					))
				}

				runImageReleaseDate, err := time.Parse(time.RFC3339, file.Config.Labels["io.buildpacks.stack.released"])
				Expect(err).NotTo(HaveOccurred())
				Expect(runImageReleaseDate).NotTo(BeZero())

				// Store the release date to compare if the date is the same as the build image on later steps
				if imageInfo.Name == "default" {
					runReleaseDate = runImageReleaseDate
				}

				Expect(file.Config.User).To(Equal("1001:1000"))

				Expect(image).To(SatisfyAll(
					HaveFileWithContent("/etc/group", ContainSubstring("cnb:x:1000:")),
					HaveFileWithContent("/etc/passwd", ContainSubstring("cnb:x:1001:1000::/home/cnb:/bin/bash")),
					HaveDirectory("/home/cnb"),
				))

				Expect(image).To(HaveFileWithContent("/etc/os-release", SatisfyAll(
					ContainLines(MatchRegexp(`PRETTY_NAME=\"Red Hat Enterprise Linux 8\.\d+ \(Ootpa\)\"`)),
					ContainSubstring(`HOME_URL="https://github.com/paketo-community/ubi-base-stack"`),
					ContainSubstring(`SUPPORT_URL="https://github.com/paketo-community/ubi-base-stack/blob/main/README.md"`),
					ContainSubstring(`BUG_REPORT_URL="https://github.com/paketo-community/ubi-base-stack/issues/new"`),
				)))
			})
		}

		Expect(runReleaseDate).To(Equal(buildReleaseDate))
	})
}

func getImageIndexAndManifests(tmpDir string, ociImageFilePath string) (index v1.ImageIndex, manifests []v1.Descriptor, err error) {

	dir := filepath.Join(tmpDir, uuid.New().String())
	err = os.Mkdir(dir, os.ModePerm)
	if err != nil {
		return nil, []v1.Descriptor{}, err
	}

	archive, err := os.Open(ociImageFilePath)
	if err != nil {
		return nil, []v1.Descriptor{}, err
	}
	defer archive.Close()

	err = vacation.NewArchive(archive).Decompress(dir)
	if err != nil {
		return nil, []v1.Descriptor{}, err
	}

	path, err := layout.FromPath(dir)
	if err != nil {
		return nil, []v1.Descriptor{}, err
	}

	index, err = path.ImageIndex()
	if err != nil {
		return nil, []v1.Descriptor{}, err
	}

	indexManifest, err := index.IndexManifest()
	if err != nil {
		return nil, []v1.Descriptor{}, err
	}

	return index, indexManifest.Manifests, nil
}
