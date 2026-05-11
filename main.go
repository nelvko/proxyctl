/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"context"
	"os"

	"charm.land/fang/v2"
	"github.com/nelvko/proxyctl/cmd"
	_ "github.com/nelvko/proxyctl/cmd/sub"
)

func main() {
	if err := fang.Execute(
		context.Background(),
		cmd.RootCmd,
		fang.WithNotifySignal(os.Interrupt, os.Kill),
	); err != nil {
		os.Exit(1)
	}

}
