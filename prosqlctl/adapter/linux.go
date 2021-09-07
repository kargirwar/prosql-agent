// +build linux

package adapter

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"text/template"

	utils "github.com/kargirwar/prosql-agent/release/utils"
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

func StartAgent() {
	//create unit file and use systemctl to start agent
	fmt.Println("Creating unit ...")
	data := struct {
		Program    string
		WorkingDir string
	}{
		Program:    "prosql-agent",
		WorkingDir: utils.GetCwd(),
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

func CopyAgent() {
	fmt.Println("Copying agent to /usr/local/bin ...")
	program := utils.GetCwd() + "/prosql-agent"
	//copy executable to /usr/local/bin
	cpCmd := exec.Command("cp", program, "/usr/local/bin")
	err := cpCmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func DelAgent() {
	fmt.Println("Deleting agent from /usr/local/bin ...")
	//copy executable to /usr/local/bin
	cmd := exec.Command("rm", "-f", "/usr/local/bin/prosql-agent")
	err := cmd.Run()

	if err != nil {
		//can't do much about error here
		log.Println(err)
	}
}

func StopAgent() {
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

func UpdateAgent() {
}
