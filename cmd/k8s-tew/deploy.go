package main

import (
	"os"
	"path"

	"github.com/darxkies/k8s-tew/deployment"
	"github.com/darxkies/k8s-tew/utils"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var identityFile string
var commandRetries uint

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy assets to a remote cluster",
	Long:  "Deploy assets to a remote cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if error := Bootstrap(false); error != nil {
			log.WithFields(log.Fields{"error": error}).Error("deploy failed")

			os.Exit(-1)
		}

		utils.SetProgressSteps(deployment.Steps(_config) + 1)

		utils.ShowProgress()

		if error := deployment.Deploy(_config, identityFile); error != nil {
			log.WithFields(log.Fields{"error": error}).Error("deploy failed")

			os.Exit(-2)
		}

		if error := deployment.Setup(_config, commandRetries); error != nil {
			log.WithFields(log.Fields{"error": error}).Error("setup failed")

			os.Exit(-3)
		}

		utils.HideProgress()

		log.Info("done")
	},
}

func init() {
	deployCmd.Flags().StringVarP(&identityFile, "identity-file", "i", path.Join(os.Getenv("HOME"), ".ssh/id_rsa"), "SSH identity file")
	deployCmd.Flags().UintVarP(&commandRetries, "command-retries", "r", 300, "The count of command retries during the setup")
	RootCmd.AddCommand(deployCmd)
}
