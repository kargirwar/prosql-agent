package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

const UNIT = `
[Unit]
Description=prosql-agent for prosql.io
[Install]
WantedBy=multi-user.target
[Service]
Type=simple
ExecStart={{.Program}}
WorkingDirectory={{.WorkingDir}}
Restart=always
RestartSec=5
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=%n
`

func main() {
	var install = flag.Bool("install", false, "Install prosql-agent on your system")
	var help = flag.Bool("help", false, "Show help message")
	var uninstall = flag.Bool("uninstall", false, "Uninstall prosql-agent from your system")
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
		copyAgent()
		startAgent()
		fmt.Println("Installed successfully! ")
		return
	}

	if *uninstall {
		delAgent()
		stopAgent()
		fmt.Println("Done.")
		return
	}
}

func startAgent() {
	//create unit file and use systemctl to start agent
	fmt.Println("Creating unit ...")
	data := struct {
		Program    string
		WorkingDir string
	}{
		Program:    "prosql-agent",
		WorkingDir: getCwd(),
	}

	unit := fmt.Sprintf("/etc/systemd/system/prosql-agent.service")
	f, err := os.Create(unit)

	t := template.Must(template.New("unit").Parse(UNIT))
	err = t.Execute(f, data)
	if err != nil {
		log.Fatalf("Unable to create unit file: %s", err)
	}
	fmt.Println("Reloading serivces...")

	cmd := exec.Command("systemctl", "daemon-reload")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Enabling prosql-agent...")
	cmd = exec.Command("systemctl", "enable", "prosql-agent.service")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Starting prosql-agent...")
	cmd = exec.Command("systemctl", "start", "prosql-agent.service")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func getCwd() string {
	//get current working dir
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return dir
}

func copyAgent() {
	fmt.Println("Copying agent to /usr/local/bin ...")
	program := getCwd() + "/prosql-agent"
	//copy executable to /usr/local/bin
	cpCmd := exec.Command("cp", program, "/usr/local/bin")
	err := cpCmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func delAgent() {
	fmt.Println("Deleting agent from /usr/local/bin ...")
	//copy executable to /usr/local/bin
	cmd := exec.Command("rm", "-f", "/usr/local/bin/prosql-agent")
	err := cmd.Run()

	if err != nil {
		//can't do much about error here
		log.Println(err)
	}
}

func stopAgent() {
	fmt.Println("Stopping agent ...")
	cmd := exec.Command("systemctl", "stop", "prosql-agent.service")
	err := cmd.Run()
	if err != nil {
		//can't do much about error here
		log.Println(err)
	}

	fmt.Println("Deleting unit ...")
	cmd = exec.Command("rm", "-f", "/etc/systemd/system/prosql-agent.service")
	err = cmd.Run()

	if err != nil {
		//can't do much about error here
		log.Println(err)
	}
}
