package main

import (
	"bytes"
	"io"
	"os"

	"get.porter.sh/plugin/kubernetes/pkg/kubernetes"
	"github.com/spf13/cobra"
)

func main() {
	in := getInput()
	cmd := buildRootCommand(in)
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCommand(in io.Reader) *cobra.Command {
	m := kubernetes.New()
	m.In = in

	cmd := &cobra.Command{
		Use:   "kubernetes",
		Short: "Kubernetes plugin for Porter",
	}

	cmd.PersistentFlags().BoolVar(&m.Debug, "debug", false, "Enable debug logging")

	cmd.AddCommand(buildVersionCommand(m))
	cmd.AddCommand(buildRunCommand(m))

	return cmd
}

func getInput() io.Reader {
	s, _ := os.Stdin.Stat()
	if (s.Mode() & os.ModeCharDevice) == 0 {
		return os.Stdin
	}

	return &bytes.Buffer{}
}
