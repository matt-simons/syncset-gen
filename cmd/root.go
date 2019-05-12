package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"github.com/matt-simons/ss/pkg"
	"github.com/openshift/hive/pkg/apis/hive/v1alpha1"
	"github.com/spf13/cobra"
)

func init() {
	viewCmd.Flags().StringVarP(&selector, "selector", "s", "", "The selector key/value pair used to create a SelectorSyncSet")
	viewCmd.Flags().StringVarP(&clusterName, "cluster-name", "c", "", "The cluster name used to create a SyncSet")
	viewCmd.Flags().StringVarP(&path, "path", "p", ".", "The path of the manifest files to use")
	RootCmd.AddCommand(viewCmd)
}

var selector, clusterName, path, name string

var RootCmd = &cobra.Command{
	Use:   "ss",
	Short: "SyncSet/SelectorSyncSet generator.",
	Long:  ``,
}

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Parses a manifest directory and prints a SyncSet/SelectorSyncSet representation of the objects it contains.",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if selector == "" && clusterName == "" {
			return errors.New("one of --selector or --cluster-name must be specified")
		}
		if selector != "" && clusterName != "" {
			return errors.New("only one of --selector or --cluster-name can be specified")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if clusterName != "" {
			var ss v1alpha1.SyncSet
			ss = pkg.CreateSyncSet(args[0], clusterName, path)
			j, err := json.MarshalIndent(&ss, "", "    ")
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			fmt.Printf("%s\n\n", string(j))
		} else {
			var ss v1alpha1.SelectorSyncSet
			ss = pkg.CreateSelectorSyncSet(args[0], selector, path)
			j, err := json.MarshalIndent(&ss, "", "    ")
			if err != nil {
				log.Fatalf("error: %v", err)
			}
			fmt.Printf("%s\n\n", string(j))
		}
	},
}
