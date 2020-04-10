package manifestbuilder

import (
	"testing"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/model"
)

// Builds Manifest objects for testing.
//
// To create well-formed manifests, we want to make sure that:
// - The relationships between targets are internally consistent
//   (e.g., if there's an ImageTarget and a K8sTarget in the manifest, then
//    the K8sTarget should depend on the ImageTarget).
// - Any filepaths in the manifest are scoped to the
//   test directory (e.g., we're not trying to watch random directories
//   outside the test environment).

type ManifestBuilder struct {
	f    Fixture
	name model.ManifestName

	k8sYAML         string
	k8sPodSelectors []labels.Selector
	dcConfigPaths   []string
	localCmd        string
	localServeCmd   string
	localDeps       []string
	resourceDeps    []string

	iTargets []model.ImageTarget
}

func New(f Fixture, name model.ManifestName) ManifestBuilder {
	return ManifestBuilder{
		f:    f,
		name: name,
	}
}

func (b ManifestBuilder) WithK8sYAML(yaml string) ManifestBuilder {
	b.k8sYAML = yaml
	return b
}

func (b ManifestBuilder) WithK8sPodSelectors(podSelectors []labels.Selector) ManifestBuilder {
	b.k8sPodSelectors = podSelectors
	return b
}

func (b ManifestBuilder) WithDockerCompose() ManifestBuilder {
	b.dcConfigPaths = []string{b.f.JoinPath("docker-compose.yml")}
	return b
}

func (b ManifestBuilder) DCConfigYaml() string {
	conf := dockercompose.ServiceConfig{}
	deployedImgs := deployedImageSet(b.iTargets)
	if len(deployedImgs) > 1 {
		panic("can't have a docker compose config with multiple top-level images")
	}
	if len(deployedImgs) == 1 {
		for _, iTarg := range deployedImgs {
			conf["image"] = iTarg.Refs.LocalRef()
			if iTarg.IsDockerBuild() {
				db := iTarg.DockerBuildInfo()
				conf["build"] = map[string]string{"context": db.BuildPath}
				// TODO: ability to set build.Dockerfile
			}
		}
	} else {
		conf["image"] = "some-great-image"
	}
	return conf.SerializeYAML()
}

func (b ManifestBuilder) WithLocalResource(cmd string, deps []string) ManifestBuilder {
	b.localCmd = cmd
	b.localDeps = deps
	return b
}

func (b ManifestBuilder) WithLocalServeCmd(cmd string) ManifestBuilder {
	b.localServeCmd = cmd
	return b
}

func (b ManifestBuilder) WithImageTarget(iTarg model.ImageTarget) ManifestBuilder {
	b.iTargets = append(b.iTargets, iTarg)
	return b
}

func (b ManifestBuilder) WithImageTargets(iTargs ...model.ImageTarget) ManifestBuilder {
	b.iTargets = append(b.iTargets, iTargs...)
	return b
}

func (b ManifestBuilder) WithLiveUpdate(lu model.LiveUpdate) ManifestBuilder {
	return b.WithLiveUpdateAtIndex(lu, 0)
}

func (b ManifestBuilder) WithLiveUpdateAtIndex(lu model.LiveUpdate, index int) ManifestBuilder {
	if len(b.iTargets) <= index {
		b.f.T().Fatalf("WithLiveUpdateAtIndex: index %d out of range -- (manifestBuilder has %d image targets)", index, len(b.iTargets))
	}

	iTarg := b.iTargets[index]
	switch bd := iTarg.BuildDetails.(type) {
	case model.DockerBuild:
		bd.LiveUpdate = lu
		b.iTargets[index] = iTarg.WithBuildDetails(bd)
	case model.CustomBuild:
		bd.LiveUpdate = lu
		b.iTargets[index] = iTarg.WithBuildDetails(bd)
	default:
		b.f.T().Fatalf("unrecognized buildDetails type: %v", bd)
	}
	return b
}

func (b ManifestBuilder) WithResourceDeps(deps ...string) ManifestBuilder {
	b.resourceDeps = deps
	return b
}

func (b ManifestBuilder) Build() model.Manifest {
	var rds []model.ManifestName
	for _, dep := range b.resourceDeps {
		rds = append(rds, model.ManifestName(dep))
	}

	if b.k8sYAML != "" {
		k8sTarget := k8s.MustTarget(model.TargetName(b.name), b.k8sYAML)
		k8sTarget.ExtraPodSelectors = b.k8sPodSelectors
		return assembleK8s(
			model.Manifest{Name: b.name, ResourceDependencies: rds},
			k8sTarget,
			b.iTargets...)
	}

	if len(b.dcConfigPaths) > 0 {
		return assembleDC(
			model.Manifest{Name: b.name, ResourceDependencies: rds},
			model.DockerComposeTarget{
				Name:        model.TargetName(b.name),
				ConfigPaths: b.dcConfigPaths,
				ConfigYAML:  b.DCConfigYaml(),
			},
			b.iTargets...)
	}

	if b.localCmd != "" || b.localServeCmd != "" {
		lt := model.NewLocalTarget(
			model.TargetName(b.name),
			model.ToShellCmd(b.localCmd),
			model.ToShellCmd(b.localServeCmd),
			b.localDeps,
			b.f.Path())
		return model.Manifest{Name: b.name, ResourceDependencies: rds}.WithDeployTarget(lt)
	}

	b.f.T().Fatalf("No deploy target specified: %s", b.name)
	return model.Manifest{}
}

func deployedImageSet(iTargets []model.ImageTarget) map[model.TargetID]model.ImageTarget {
	// assume that images on which another image depends are base images,
	// images, i.e. not deployed directly
	baseImages := make(map[model.TargetID]bool)
	for _, iTarget := range iTargets {
		for _, id := range iTarget.DependencyIDs() {
			baseImages[id] = true
		}
	}

	deployed := make(map[model.TargetID]model.ImageTarget)
	for _, iTarget := range iTargets {
		if !baseImages[iTarget.ID()] {
			deployed[iTarget.ID()] = iTarget
		}
	}

	return deployed
}

type Fixture interface {
	T() testing.TB
	Path() string
	JoinPath(ps ...string) string
	MkdirAll(p string)
}
