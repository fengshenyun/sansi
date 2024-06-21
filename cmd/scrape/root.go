package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const (
	cliName        = "sansi"
	cliDescription = "A simple command line client for scrape sansi.com comics."
)

var (
	rootCmd = &cobra.Command{
		Use:        cliName,
		Short:      cliDescription,
		SuggestFor: []string{"sansi"},
	}
)

var (
	globalFlags = GlobalFlags{}
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&globalFlags.Debug, "debug", false, "enable logging")

	rootCmd.AddCommand(
		NewScrapeCommand(),
	)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
