package main

import (
	"flag"
	"fmt"

	"github.com/a0xAi/kubeve/ui"
)

func main() {
	version := "0.4.2"

	showVersion := flag.Bool("v", false, "print version")
	help := flag.Bool("h", false, "show help")
	namespace := flag.String("n", "", "Kubernetes namespace to use")
	flag.Parse()

	if *help {
		flag.Usage()
		return
	}
	if *showVersion {
		fmt.Println(version)
		return
	}

	ui.StartUI(version, *namespace)
}
