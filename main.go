package main

import (
	"log"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/mohammadne/caas-operator/cmd"
)

func main() {
	const description = "Container as a Service"
	root := &cobra.Command{Short: description}

	root.AddCommand(
		cmd.Server{}.Command(),
	)

	if err := root.Execute(); err != nil {
		log.Fatal("failed to execute root command", zap.Error(err))
	}
}
