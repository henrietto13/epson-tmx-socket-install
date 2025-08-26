package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// socketFileContent holds the configuration for the systemd socket unit.
// It listens on all network interfaces on TCP port 9100.
const socketFileContent = `[Unit]
Description=Epson Printer Socket

[Socket]
ListenStream=0.0.0.0:9100
Accept=yes

[Install]
WantedBy=sockets.target
`

// serviceFileContent holds the configuration for the systemd service unit.
// This is a template service that gets instantiated for each incoming connection.
// It uses 'tee' to pipe the incoming data to the printer and /dev/null,
// which resolved a timing issue in the original setup.
const serviceFileContent = `[Unit]
Description=Epson Printer Service

[Service]
ExecStart=-/usr/bin/tee /dev/null > /dev/usb/lp0
StandardInput=socket
`

func main() {
	fmt.Println("Starting Epson Printer Service Setup...")

	// --- Step 1: Check for root privileges ---
	// This is critical because we need to write files to /etc/systemd/system
	// and run systemctl commands, which require elevated permissions.
	if os.Geteuid() != 0 {
		log.Fatal("This program must be run as root or with sudo.")
	}
	fmt.Println("✓ Root privileges confirmed.")

	// --- Step 2: Define file paths ---
	socketFilePath := "/etc/systemd/system/epson-printer.socket"
	serviceFilePath := "/etc/systemd/system/epson-printer@.service"

	// --- Step 3: Write the systemd unit files ---
	err := os.WriteFile(socketFilePath, []byte(socketFileContent), 0644)
	if err != nil {
		log.Fatalf("Failed to write socket file: %v", err)
	}
	fmt.Printf("✓ Successfully created %s\n", socketFilePath)

	err = os.WriteFile(serviceFilePath, []byte(serviceFileContent), 0644)
	if err != nil {
		log.Fatalf("Failed to write service file: %v", err)
	}
	fmt.Printf("✓ Successfully created %s\n", serviceFilePath)

	// --- Step 4: Run systemctl commands to enable and start the service ---
	// We run these commands in sequence to make systemd aware of the new files,
	// enable the socket to start on boot, and start it immediately.
	commands := [][]string{
		{"systemctl", "daemon-reload"},
		{"systemctl", "enable", "--now", "epson-printer.socket"},
		{"systemctl", "restart", "epson-printer.socket"}, // Use restart to ensure it's fresh
	}

	for _, cmdArgs := range commands {
		cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
		fmt.Printf("Executing: %s...\n", strings.Join(cmd.Args, " "))
		output, err := cmd.CombinedOutput() // CombinedOutput gets both stdout and stderr
		if err != nil {
			log.Fatalf("Failed to execute command '%s': %v\nOutput: %s", strings.Join(cmd.Args, " "), err, string(output))
		}
		fmt.Printf("✓ Command successful.\n")
	}

	fmt.Println("\n🎉 Setup complete! The epson printer socket is active and enabled.")
	fmt.Println("The PC is now ready to accept print jobs on TCP port 9100.")
}
