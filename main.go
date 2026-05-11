/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"context"

	"charm.land/fang/v2"
	"github.com/nelvko/proxyctl/cmd"
	_ "github.com/nelvko/proxyctl/cmd/sub"
)

func main() {
	fang.Execute(context.Background(), cmd.RootCmd)
}
