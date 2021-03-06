package kubectl

import (
	"errors"
	"os/exec"
	"strings"

	"k8s.io/client-go/kubernetes"

	"github.com/devspace-cloud/devspace/pkg/devspace/config/configutil"
	"github.com/devspace-cloud/devspace/pkg/devspace/config/generated"
	"github.com/devspace-cloud/devspace/pkg/devspace/deploy"
	"github.com/devspace-cloud/devspace/pkg/devspace/deploy/kubectl/walk"

	v1 "github.com/devspace-cloud/devspace/pkg/devspace/config/versions/latest"
	"github.com/devspace-cloud/devspace/pkg/util/log"
)

// DeployConfig holds the necessary information for kubectl deployment
type DeployConfig struct {
	KubeClient *kubernetes.Clientset // This is not used yet, however the plan is to use it instead of calling kubectl via cmd
	Name       string
	CmdPath    string
	Context    string
	Namespace  string
	Manifests  []string
	Log        log.Logger
}

// New creates a new deploy config for kubectl
func New(kubectl *kubernetes.Clientset, deployConfig *v1.DeploymentConfig, log log.Logger) (*DeployConfig, error) {
	if deployConfig.Kubectl == nil {
		return nil, errors.New("Error creating kubectl deploy config: kubectl is nil")
	}
	if deployConfig.Kubectl.Manifests == nil {
		return nil, errors.New("No manifests defined for kubectl deploy")
	}

	config := configutil.GetConfig()

	context := ""
	if config.Cluster != nil && config.Cluster.KubeContext != nil {
		context = *config.Cluster.KubeContext
	}

	namespace, err := configutil.GetDefaultNamespace(config)
	if err != nil {
		return nil, err
	}
	if deployConfig.Namespace != nil && *deployConfig.Namespace != "" {
		namespace = *deployConfig.Namespace
	}

	cmdPath := "kubectl"
	if deployConfig.Kubectl.CmdPath != nil {
		cmdPath = *deployConfig.Kubectl.CmdPath
	}

	manifests := []string{}
	for _, manifest := range *deployConfig.Kubectl.Manifests {
		manifests = append(manifests, *manifest)
	}

	return &DeployConfig{
		Name:       *deployConfig.Name,
		KubeClient: kubectl,
		CmdPath:    cmdPath,
		Context:    context,
		Namespace:  namespace,
		Manifests:  manifests,
		Log:        log,
	}, nil
}

// Status prints the status of all matched manifests from kubernetes
func (d *DeployConfig) Status() (*deploy.StatusResult, error) {
	// TODO: parse kubectl get output into the required string array
	manifests := strings.Join(d.Manifests, ",")
	if len(manifests) > 20 {
		manifests = manifests[:20] + "..."
	}

	return &deploy.StatusResult{
		Name:   d.Name,
		Type:   "Manifests",
		Target: manifests,
		Status: "N/A",
	}, nil
}

// Delete deletes all matched manifests from kubernetes
func (d *DeployConfig) Delete() error {
	d.Log.StartWait("Loading manifests")
	manifests, err := loadManifests(d.Manifests, d.Log)
	if err != nil {
		return err
	}

	joinedManifests, err := joinManifests(manifests)
	if err != nil {
		return err
	}

	stringReader := strings.NewReader(joinedManifests)
	args := d.getCmdArgs("delete", "--ignore-not-found=true")

	cmd := exec.Command(d.CmdPath, args...)

	cmd.Stdin = stringReader
	cmd.Stdout = d.Log
	cmd.Stderr = d.Log

	d.Log.StartWait("Deleting manifests with kubectl")
	defer d.Log.StopWait()
	return cmd.Run()
}

// Deploy deploys all specified manifests via kubectl apply and adds to the specified image names the corresponding tags
func (d *DeployConfig) Deploy(generatedConfig *generated.Config, isDev, forceDeploy bool) error {
	d.Log.StartWait("Loading manifests")
	manifests, err := loadManifests(d.Manifests, d.Log)
	if err != nil {
		return err
	}

	activeConfig := generatedConfig.GetActive().Deploy
	if isDev {
		activeConfig = generatedConfig.GetActive().Dev
	}

	for _, manifest := range manifests {
		replaceManifest(manifest, activeConfig.ImageTags)
	}

	joinedManifests, err := joinManifests(manifests)
	if err != nil {
		return err
	}

	stringReader := strings.NewReader(joinedManifests)
	args := d.getCmdArgs("apply", "--force")

	cmd := exec.Command(d.CmdPath, args...)

	cmd.Stdin = stringReader
	cmd.Stdout = d.Log
	cmd.Stderr = d.Log

	d.Log.StartWait("Applying manifests with kubectl")
	defer d.Log.StopWait()
	return cmd.Run()
}

func (d *DeployConfig) getCmdArgs(method string, additionalArgs ...string) []string {
	args := []string{}

	if d.Context != "" {
		args = append(args, "--context", d.Context)
	}
	if d.Namespace != "" {
		args = append(args, "-n", d.Namespace)
	}

	args = append(args, method)

	if additionalArgs != nil {
		args = append(args, additionalArgs...)
	}

	args = append(args, "-f", "-")

	return args
}

func replaceManifest(manifest Manifest, tags map[string]string) {
	match := func(path, key, value string) bool {
		if key == "image" {
			if _, ok := tags[value]; ok {
				return true
			}
		}

		return false
	}

	replace := func(path, value string) interface{} {
		return value + ":" + tags[value]
	}

	walk.Walk(map[interface{}]interface{}(manifest), match, replace)
}
