package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getlantern/systray"
)

var ignoreApps = map[string]bool{
	"rapportd":  true,
	"ControlCe": true,
	"sharingd":  true,
	"coreaudio": true,
	"kdc":       true,
	"IdentityS": true,
	"systemmd":  true,
	"loginwind": true,
	"AirPlayUX": true,
	"Reminders": true,
	"Siri":      true,
	"assistant": true,
}

var (
	httpCache = make(map[string]int) // Store HTTP status code, 0 if invalid
	cacheMu   sync.Mutex
)

func checkHTTPServer(pid, command, port string) int {
	if ignoreApps[command] {
		return 0
	}

	key := pid + ":" + port
	cacheMu.Lock()
	status, exists := httpCache[key]
	cacheMu.Unlock()

	if exists {
		return status
	}

	client := http.Client{Timeout: 1 * time.Second}
	req, _ := http.NewRequest("HEAD", "http://localhost:"+port, nil)
	resp, err := client.Do(req)

	status = 0
	if err == nil {
		resp.Body.Close()
		// Filter out endpoints that actively refuse the connection
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
			status = resp.StatusCode
			if status == 0 {
				status = 200 // Default to valid if weird 0 status
			}
		}
	} else if strings.Contains(err.Error(), "malformed HTTP response") || strings.Contains(err.Error(), "server gave HTTP response to HTTPS client") {
		status = 200 // Treat as valid
	}

	cacheMu.Lock()
	httpCache[key] = status
	cacheMu.Unlock()

	return status
}

type Process struct {
	PID        string
	Command    string
	Port       string
	PWD        string
	StatusCode int
}

func main() {
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(iconData)
	systray.SetTooltip("Open HTTP Ports")

	mQuit := systray.AddMenuItem("Quit", "Quit the whole app")
	systray.AddSeparator()

	go func() {
		<-mQuit.ClickedCh
		systray.Quit()
	}()

	// We no longer pass items parameter since currentMenu is global
	go func() {
		for {
			updateMenu()
			time.Sleep(10 * time.Second)
		}
	}()
}

func onExit() {
	// clean up here
}

type ProcessMenuItem struct {
	Parent *systray.MenuItem
	Open   *systray.MenuItem
	Copy   *systray.MenuItem
	Stop   *systray.MenuItem
	Proc   Process
}

// Map to keep track of dynamic menu items
var currentMenu []*ProcessMenuItem

func updateMenu() {
	procs, err := getOpenPorts()
	if err != nil {
		fmt.Println("Error getting open ports:", err)
		return
	}

	// Group into normal and 400+ processes
	var normalProcs []Process
	var errorProcs []Process

	for _, p := range procs {
		if p.StatusCode >= 400 {
			errorProcs = append(errorProcs, p)
		} else {
			normalProcs = append(normalProcs, p)
		}
	}

	allSortedProcs := normalProcs
	if len(errorProcs) > 0 {
		allSortedProcs = append(allSortedProcs, Process{PID: "SEP", Port: "0"})
		allSortedProcs = append(allSortedProcs, errorProcs...)
	}

	// Unfortunately github.com/getlantern/systray doesn't support Remove/Delete.
	// The only way to reorder or insert a separator dynamically is to hide everything,
	// and allocate new elements if we need more, reusing existing hidden slots linearly.

	// Ensure we have enough menu item slots
	for len(currentMenu) < len(allSortedProcs) {
		parent := systray.AddMenuItem("", "")
		openItem := parent.AddSubMenuItem("Open in Browser", "Open this port in your default browser")
		copyItem := parent.AddSubMenuItem("Copy URL", "Copy the localhost URL to clipboard")
		stopItem := parent.AddSubMenuItem("Stop Process", "Force quit this process")

		pmi := &ProcessMenuItem{
			Parent: parent,
			Open:   openItem,
			Copy:   copyItem,
			Stop:   stopItem,
		}
		currentMenu = append(currentMenu, pmi)

		// Handler
		go func(item *ProcessMenuItem) {
			for {
				select {
				case <-item.Open.ClickedCh:
					if item.Proc.PID != "SEP" && item.Proc.PID != "" {
						openBrowser("http://localhost:" + item.Proc.Port)
					}
				case <-item.Copy.ClickedCh:
					if item.Proc.PID != "SEP" && item.Proc.PID != "" {
						copyToClipboard("http://localhost:" + item.Proc.Port)
					}
				case <-item.Stop.ClickedCh:
					if item.Proc.PID != "SEP" && item.Proc.PID != "" {
						stopProcess(item.Proc.PID)
					}
				}
			}
		}(pmi)
	}

	// Update the slots
	for i, slot := range currentMenu {
		if i < len(allSortedProcs) {
			p := allSortedProcs[i]
			slot.Proc = p

			if p.PID == "SEP" {
				slot.Parent.SetTitle("───────────────")
				slot.Parent.Disable()
				slot.Open.Hide()
				slot.Copy.Hide()
				slot.Stop.Hide()
			} else {
				title := fmt.Sprintf("[%s] %s", p.Port, p.Command)
				if p.PWD != "" {
					title += fmt.Sprintf(" {in %s}", p.PWD)
				}
				if p.StatusCode >= 400 {
					title = "⚠️ " + title + fmt.Sprintf(" (HTTP %d)", p.StatusCode)
				}

				slot.Parent.SetTitle(title)
				slot.Parent.Enable()
				slot.Open.Show()
				slot.Copy.Show()
				slot.Stop.Show()
			}
			slot.Parent.Show()
		} else {
			// Slot is unused
			slot.Parent.Hide()
		}
	}
}

func getOpenPorts() ([]Process, error) {
	cmd := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n", "-l")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 1 && out.Len() == 0 {
				// Exit code 1 with no output means no open ports found, which is fine
				return nil, nil
			}
		}
		return nil, err
	}

	procs, err := parseLsof(out.String())
	if err != nil {
		return nil, err
	}

	// Filter for actual HTTP servers concurrently
	var filtered []Process
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, p := range procs {
		wg.Add(1)
		go func(proc Process) {
			defer wg.Done()

			status := checkHTTPServer(proc.PID, proc.Command, proc.Port)
			// Status 0 means not an HTTP server or actively ignored.
			// Let's include 400+ errors as requested but hide the pure noise
			if status > 0 {
				proc.StatusCode = status
				mu.Lock()
				filtered = append(filtered, proc)
				mu.Unlock()
			}
		}(p)
	}
	wg.Wait()

	// Sort the filtered list again since concurrent append randomizes it
	sort.Slice(filtered, func(i, j int) bool {
		pi, _ := strconv.Atoi(filtered[i].Port)
		pj, _ := strconv.Atoi(filtered[j].Port)
		return pi < pj
	})

	return filtered, nil
}

func parseLsof(output string) ([]Process, error) {
	var procs []Process
	scanner := bufio.NewScanner(strings.NewReader(output))

	// Skip header
	if scanner.Scan() {
		_ = scanner.Text()
	}

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		command := fields[0]
		pid := fields[1]

		// Parse the "TCP *:8080 (LISTEN)" OR "TCP 127.0.0.1:8080 (LISTEN)" part
		// The 9th field is usually the node name eg: *:8080 or 127.0.0.1:8080 or [::1]:8080
		node := fields[8]
		portIdx := strings.LastIndex(node, ":")
		if portIdx == -1 {
			continue
		}
		port := node[portIdx+1:]

		// Optionally skip standard ports if required, but we'll include everything
		pwd := getPwdForPid(pid)

		procs = append(procs, Process{
			PID:     pid,
			Command: command,
			Port:    port,
			PWD:     pwd,
		})
	}

	// Sort by port
	sort.Slice(procs, func(i, j int) bool {
		pi, _ := strconv.Atoi(procs[i].Port)
		pj, _ := strconv.Atoi(procs[j].Port)
		return pi < pj
	})

	return procs, scanner.Err()
}

func getPwdForPid(pid string) string {
	cmd := exec.Command("lsof", "-p", pid, "-a", "-d", "cwd", "-F", "n")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ""
	}

	// Output format is:
	// p<PID>
	// fcwd
	// n<PWD>

	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "n") {
			return line[1:]
		}
	}
	return ""
}

func copyToClipboard(text string) {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = strings.NewReader(text)
	_ = cmd.Run()
}

func openBrowser(url string) {
	cmd := exec.Command("open", url)
	_ = cmd.Run()
}

func stopProcess(pid string) {
	cmd := exec.Command("kill", "-9", pid)
	_ = cmd.Run()
	// Menu will update naturally on the next refresh tick
}
