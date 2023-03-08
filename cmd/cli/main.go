package main

import "github.com/spf13/cobra"

func main() {
	var rootCmd = &cobra.Command{Use: "app"}
	rootCmd.Execute()
}
