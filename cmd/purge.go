package cmd

import (
	"strings"

	"github.com/devspace-cloud/devspace/pkg/devspace/config/configutil"
	deploy "github.com/devspace-cloud/devspace/pkg/devspace/deploy/util"
	"github.com/devspace-cloud/devspace/pkg/devspace/kubectl"
	"github.com/devspace-cloud/devspace/pkg/util/log"

	"github.com/spf13/cobra"
)

// PurgeCmd holds the required data for the purge cmd
type PurgeCmd struct {
	Deployments string
}

// NewPurgeCmd creates a new purge command
func NewPurgeCmd() *cobra.Command {
	cmd := &PurgeCmd{}

	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Delete deployed resources",
		Long: `
#######################################################
################### devspace purge ####################
#######################################################
Deletes the deployed kuberenetes resources:

devspace purge
devspace purge -d my-deployment
#######################################################`,
		Args: cobra.NoArgs,
		Run:  cmd.Run,
	}

	purgeCmd.Flags().StringVarP(&cmd.Deployments, "deployments", "d", "", "The deployment to delete (You can specify multiple deployments comma-separated, e.g. devspace-default,devspace-database etc.)")

	return purgeCmd
}

// Run executes the purge command logic
func (cmd *PurgeCmd) Run(cobraCmd *cobra.Command, args []string) {
	// Set config root
	configExists, err := configutil.SetDevSpaceRoot()
	if err != nil {
		log.Fatal(err)
	}
	if !configExists {
		log.Fatal("Couldn't find any devspace configuration. Please run `devspace init`")
	}

	log.StartFileLogging()

	kubectl, err := kubectl.NewClient()
	if err != nil {
		log.Fatalf("Unable to create new kubectl client: %s", err.Error())
	}

	deployments := []string{}
	if cmd.Deployments != "" {
		deployments = strings.Split(cmd.Deployments, ",")
		for index := range deployments {
			deployments[index] = strings.TrimSpace(deployments[index])
		}
	}

	deploy.PurgeDeployments(kubectl, deployments)
}
