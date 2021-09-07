package main

import (
	"flag"
	"fmt"

	"github.com/kargirwar/prosql-agent/prosqlctl/adapter"
)

func main() {
	var install = flag.Bool("install", false, "Install agent on your system")
	var help = flag.Bool("help", false, "Show help message")
	var uninstall = flag.Bool("uninstall", false, "Uninstall prosql-agent from your system")
	var update = flag.Bool("update", false, "Update prosql-agent")
	flag.Parse()

	flag.Usage = func() {
		w := flag.CommandLine.Output()
		fmt.Fprintf(w, "prosqlctl usage:\n")
		flag.PrintDefaults()
	}

	if *help {
		flag.Usage()
		return
	}

	if flag.NFlag() == 0 {
		flag.Usage()
		return
	}

	if *install {
		installAgent()
		fmt.Println("Installed successfully!")
		return
	}

	if *uninstall {
		unInstallAgent()
		fmt.Println("Done.")
		return
	}

	if *update {
		adapter.UpdateAgent()
		fmt.Println("Updated successfully!")
	}
}

func installAgent() {
	adapter.CopyAgent()
	adapter.StartAgent()
}

func unInstallAgent() {
	adapter.DelAgent()
	adapter.StopAgent()
}
