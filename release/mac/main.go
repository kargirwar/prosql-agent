package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"text/template"
	"time"
)

const CURRENT_RELEASE = "https://raw.githubusercontent.com/kargirwar/prosql-agent/master/current-release.json"
const RELEASE_ARCHIVE = "release.zip"
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
		updateAgent()
		fmt.Println("Updated successfully!")
	}
}

func updateAgent() {
	release := getLatestRelease()

	//Download and extract
	fmt.Println("Updating to " + release.Version)
	fmt.Printf("Downloading release ..")
	downloadFile(RELEASE_ARCHIVE, release.Mac)
	fmt.Println("Done.")

	fmt.Printf("Extracting files ..")
	t := time.Now()
	now := fmt.Sprintf("%d-%02d-%02dT%02d-%02d-%02d",
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second())

	dir := "temp-" + now
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	unzip(RELEASE_ARCHIVE, dir)
	fmt.Println("Done.")

	//========================================================
	//Copy agent to current directory, uninstall current version
	//and install updated version
	copyFrom(dir)
	unInstallAgent()
	installAgent()
	//========================================================

	fmt.Printf("Cleaning up ..")
	//delete temp files
	err = os.RemoveAll(dir)
	if err != nil {
		log.Fatal(err)
	}

	//delete archive
	err = os.Remove(RELEASE_ARCHIVE)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Done.")
}

func installAgent() {
	copyAgent()
	startAgent()
}

func unInstallAgent() {
	delAgent()
	stopAgent()
}

func copyFrom(dir string) {
	program := dir + "/prosql-agent/release/mac/" + BINARY

	//copy executable to /usr/local/bin
	cmd := exec.Command("cp", program, getCwd())
	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
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
		Program: "/usr/local/bin/prosql-agent",
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

func copyAgent() {
	fmt.Println("Copying agent to /usr/local/bin ...")
	program := getCwd() + "/prosql-agent"

	//copy executable to /usr/local/bin
	cmd := exec.Command("cp", "-v", program, "/usr/local/bin")
	err := cmd.Run()
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
