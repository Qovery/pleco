package core

import (
	"github.com/Qovery/pleco/providers/aws"
	"github.com/Qovery/pleco/providers/k8s"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"sync"
)

func StartDaemon(disableDryRun bool, interval int64, cmd *cobra.Command) {
	var wg sync.WaitGroup
	dryRun := true
	if disableDryRun {
		dryRun = false
	} else {
		log.Info("Dry run mode enabled")
	}

	checkEnvVars(cmd)

	// run Kubernetes check
	k8s.RunPlecoKubernetes(cmd, interval, dryRun, &wg)

	// run AWS checks
	regions, _ := cmd.Flags().GetStringSlice("aws-regions")
	aws.RunPlecoAWS(cmd, regions, interval, dryRun, &wg)

	wg.Wait()
}
