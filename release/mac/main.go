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

const BINARY = "prosql-agent"
const LABEL = "io.prosql.agent"
const PLIST = `<?xml version='1.0' encoding='UTF-8'?>
<!DOCTYPE plist PUBLIC \"-//Apple Computer//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\" >
<plist version='1.0'>
  <dict>
    <key>Label</key><string>{{.Label}}</string>
    <key>Program</key><string>{{.Program}}</string>
	<key>StandardOutPath</key><string>/tmp/{{.Label}}.out.log</string>
	<key>StandardErrorPath</key><string>/tmp/{{.Label}}.err.log</string>
    <key>KeepAlive</key><true/>
    <key>RunAtLoad</key><true/>
  </dict>
</plist>
`

func main() {
	var install = flag.Bool("install", false, "Install agent on your system")
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
	//create plist and use launchctrl to start service
	fmt.Println("Creating plist ...")
	data := struct {
		Label   string
		Program string
	}{
		Label:   LABEL,
		Program: "/usr/local/bin/" + BINARY,
	}

	plist := fmt.Sprintf("%s/Library/LaunchAgents/%s.plist", os.Getenv("HOME"), data.Label)
	f, err := os.Create(plist)

	t := template.Must(template.New("launchdConfig").Parse(PLIST))
	err = t.Execute(f, data)
	if err != nil {
		log.Fatalf("Template generation failed: %s", err)
	}
	fmt.Println("Starting ...")

	cmd := exec.Command("launchctl", "load", plist)
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func getProgram() string {
	//get current working dir
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Fatal(err)
	}
	return dir + "/" + BINARY
}

func copyAgent() {
	fmt.Println("Copying agent to /usr/local/bin ...")
	program := getProgram()
	//copy executable to /usr/local/bin
	cpCmd := exec.Command("cp", "-v", program, "/usr/local/bin")
	err := cpCmd.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func delAgent() {
	fmt.Println("Deleting agent from /usr/local/bin ...")
	//copy executable to /usr/local/bin
	cmd := exec.Command("rm", "-f", "/usr/local/bin/"+BINARY)
	err := cmd.Run()

	if err != nil {
		//can't do much about error here
		log.Println(err)
	}
}

func stopAgent() {
	plist := fmt.Sprintf("%s/Library/LaunchAgents/%s.plist", os.Getenv("HOME"), LABEL)

	fmt.Println("Stopping agent ...")
	cmd := exec.Command("launchctl", "unload", plist)
	err := cmd.Run()
	if err != nil {
		//can't do much about error here
		log.Println(err)
	}

	fmt.Println("Deleting plist ...")
	cmd = exec.Command("rm", "-f", plist)
	err = cmd.Run()

	if err != nil {
		//can't do much about error here
		log.Println(err)
	}
}
