package cmd

import (
	"fmt"
	"os"

	"github.com/fengshenyun/sansi/pkg/scrape"
	"github.com/spf13/cobra"
)

var (
	url      string
	rootPath string
	timeout  int
)

func NewScrapeCommand() *cobra.Command {
	ac := &cobra.Command{
		Use:   "scrape [options]",
		Short: "Scrape one web url.",
		Run:   scrapeCommandFunc,
	}

	ac.Flags().StringVar(&url, "url", "", "Scrape target url")
	ac.Flags().IntVar(&timeout, "timeout", 10, "Set connect timeout")
	ac.Flags().StringVar(&rootPath, "root-path", "./data", "Set root path")

	return ac
}

func scrapeCommandFunc(cmd *cobra.Command, args []string) {
	sc := new(scrape.Config)
	sc.Debug = globalFlags.Debug
	sc.Url = url
	sc.Timeout = timeout
	sc.RootPath = rootPath

	if err := scrape.NewWithConfig(sc).Scrape(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
