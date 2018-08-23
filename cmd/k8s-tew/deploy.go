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
var skipSetup bool

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy assets to a remote cluster",
	Long:  "Deploy assets to a remote cluster",
	Run: func(cmd *cobra.Command, args []string) {
		if error := Bootstrap(false); error != nil {
			log.WithFields(log.Fields{"error": error}).Error("Failed initializing")

			os.Exit(-1)
		}

		_deployment := deployment.NewDeployment(_config, identityFile, skipSetup, commandRetries)

		utils.SetProgressSteps(_deployment.Steps() + 1)

		utils.ShowProgress()

		if error := _deployment.Deploy(); error != nil {
			log.WithFields(log.Fields{"error": error}).Error("Failed deploying")

			os.Exit(-2)
		}

		utils.HideProgress()

		log.Info("Done")
	},
}

func init() {
	deployCmd.Flags().StringVarP(&identityFile, "identity-file", "i", path.Join(os.Getenv("HOME"), ".ssh/id_rsa"), "SSH identity file")
	deployCmd.Flags().UintVarP(&commandRetries, "command-retries", "r", 300, "The count of command retries during the setup")
	deployCmd.Flags().BoolVarP(&skipSetup, "skip-setup", "s", false, "Skip setup steps")
	RootCmd.AddCommand(deployCmd)
}
