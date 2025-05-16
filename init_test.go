package acceptance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paketo-buildpacks/occam"
	utils "github.com/paketo-buildpacks/ubi8-base-stack/internal/utils"
	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"

	. "github.com/onsi/gomega"
)

var root string
var RegistryUrl string
var DefaultStack StackImages

type StackImages struct {
	Name                    string `json:"name"`
	ConfigDir               string `json:"config_dir"`
	OutputDir               string `json:"output_dir"`
	BuildImage              string `json:"build_image"`
	RunImage                string `json:"run_image"`
	BuildReceiptFilename    string `json:"build_receipt_filename"`
	RunReceiptFilename      string `json:"run_receipt_filename"`
	CreateBuildImage        bool   `json:"create_build_image,omitempty"`
	BaseBuildContainerImage string `json:"base_build_container_image,omitempty"`
	BaseRunContainerImage   string `json:"base_run_container_image"`
	Type                    string `json:"type,omitempty"`
}

type ImagesJson struct {
	SupportUsns       bool          `json:"support_usns"`
	ReceiptsShowLimit int           `json:"receipts_show_limit"`
	StackImages       []StackImages `json:"images"`
}

var builder struct {
	imageUrl      string
	buildImageUrl string
	runImageUrl   string
}

var settings struct {
	Buildpacks struct {
		Nodejs struct {
			Online string
		}
		BuildPlan struct {
			Online string
		}
		GoDist struct {
			Online string
		}
	}

	Extensions struct {
		UbiNodejsExtension struct {
			Online string
		}
	}

	Config struct {
		BuildPlan          string `json:"build-plan"`
		UbiNodejsExtension string `json:"ubi-nodejs-extension"`
		Nodejs             string `json:"nodejs"`
		GoDist             string `json:"go-dist"`
	}

	ImagesJson ImagesJson
}

func by(_ string, f func()) { f() }

func TestAcceptance(t *testing.T) {

	var err error
	Expect := NewWithT(t).Expect

	docker := occam.NewDocker()

	RegistryUrl = os.Getenv("REGISTRY_URL")
	Expect(RegistryUrl).NotTo(Equal(""))

	root, err = filepath.Abs(".")
	Expect(err).ToNot(HaveOccurred())

	integration_json, err := os.Open("./integration.json")
	Expect(err).NotTo(HaveOccurred())

	Expect(json.NewDecoder(integration_json).Decode(&settings.Config)).To(Succeed())
	Expect(integration_json.Close()).To(Succeed())

	images_json, err := os.Open("./images.json")
	Expect(err).NotTo(HaveOccurred())

	Expect(json.NewDecoder(images_json).Decode(&settings.ImagesJson)).To(Succeed())
	Expect(images_json.Close()).To(Succeed())

	testOnlyStacksEnv := os.Getenv("TEST_ONLY_STACKS")
	var testOnlystacks []string

	if testOnlyStacksEnv != "" {
		testOnlystacks = strings.Split(testOnlyStacksEnv, " ")
	}

	if len(testOnlystacks) > 0 {
		var filteredStacks []StackImages
		for _, stack := range settings.ImagesJson.StackImages {
			for _, testStack := range testOnlystacks {
				if stack.Name == testStack {
					filteredStacks = append(filteredStacks, stack)
				}
			}
		}
		settings.ImagesJson.StackImages = filteredStacks
	}

	DefaultStack = getDefaultStack(settings.ImagesJson.StackImages)
	Expect(DefaultStack).NotTo(Equal(StackImages{}))

	buildpackStore := occam.NewBuildpackStore()

	settings.Extensions.UbiNodejsExtension.Online, err = buildpackStore.Get.
		Execute(settings.Config.UbiNodejsExtension)
	Expect(err).ToNot(HaveOccurred())

	settings.Buildpacks.Nodejs.Online, err = buildpackStore.Get.
		Execute(settings.Config.Nodejs)
	Expect(err).ToNot(HaveOccurred())

	settings.Buildpacks.BuildPlan.Online, err = buildpackStore.Get.
		Execute(settings.Config.BuildPlan)
	Expect(err).ToNot(HaveOccurred())

	settings.Buildpacks.GoDist.Online, err = buildpackStore.Get.
		Execute(settings.Config.GoDist)
	Expect(err).NotTo(HaveOccurred())

	builder.buildImageUrl, builder.runImageUrl, builder.imageUrl, err = utils.GenerateBuilder(
		filepath.Join(root, DefaultStack.OutputDir, "build.oci"),
		filepath.Join(root, DefaultStack.OutputDir, "run.oci"),
		RegistryUrl,
	)
	Expect(err).NotTo(HaveOccurred())

	SetDefaultEventuallyTimeout(120 * time.Second)

	suite := spec.New("Acceptance", spec.Report(report.Terminal{}), spec.Parallel())
	suite("Metadata", testMetadata)
	suite("NodejsStackIntegration", testNodejsStackIntegration)
	suite("buildpackIntegration", testBuildpackIntegration)
	suite.Run(t)

	/** Cleanup **/
	lifecycleImageID, err := utils.GetLifecycleImageID(docker, builder.imageUrl)
	Expect(err).NotTo(HaveOccurred())

	err = utils.RemoveImages(docker, []string{lifecycleImageID, builder.runImageUrl, builder.imageUrl})
	Expect(err).NotTo(HaveOccurred())

}

func getDefaultStack(stackImages []StackImages) StackImages {
	for _, stack := range stackImages {
		if stack.Name == "default" {
			return stack
		}
	}
	return StackImages{}
}
