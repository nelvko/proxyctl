/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"context"
	"os"
	"syscall"

	"charm.land/fang/v2"
	"github.com/nelvko/proxyctl/cmd"
)

func main() {
	if err := fang.Execute(
		context.Background(),
		cmd.NewRootCmd(),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		os.Exit(1)
	}
}
