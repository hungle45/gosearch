package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

// Package represents a single Go package from the index.
type Package struct {
	Path      string    `json:"Path"`
	Version   string    `json:"Version"`
	Timestamp time.Time `json:"Timestamp"`
}

// Model represents the state of our terminal UI application.
type model struct {
	packages       []Package
	filtered       []fuzzy.Match
	searchQuery    string
	selectedIndex  int
	loading        bool
	err            error
	quitting       bool
	viewportOffset int
	pageSize       int
	finalMessage   string
}

// Styles for the UI elements.
var (
	inputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#007bff"))

	itemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#888"))

	selectedItemStyle = lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(lipgloss.Color("#007bff")).
		Background(lipgloss.Color("#e0f2ff")).
		Bold(true)

	statusMessageStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#007bff")).
		Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		Padding(0, 1)

	successMessageStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Padding(0, 1).
		Bold(true)

	// New style for package version
	versionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#a0a0a0")). // Lighter grey color
		MarginLeft(1) // Small space from the path
)

func (m model) Init() tea.Cmd {
	return fetchPackagesCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			m.finalMessage = "Exiting Go Package Search CLI."
			return m, tea.Quit

		case "up", "k":
			if len(m.filtered) > 0 {
				m.selectedIndex--
				if m.selectedIndex < 0 {
					m.selectedIndex = len(m.filtered) - 1
				}
				m.updateViewportOffset()
			}

		case "down", "j":
			if len(m.filtered) > 0 {
				m.selectedIndex++
				if m.selectedIndex >= len(m.filtered) {
					m.selectedIndex = 0
				}
				m.updateViewportOffset()
			}

		case "enter":
			if len(m.filtered) > 0 && m.selectedIndex >= 0 && m.selectedIndex < len(m.filtered) {
				matchedPackage := m.filtered[m.selectedIndex]
				if matchedPackage.Index >= 0 && matchedPackage.Index < len(m.packages) {
					packagePath := m.packages[matchedPackage.Index].Path
					m.quitting = true
					m.finalMessage = fmt.Sprintf("'%s' copied to clipboard!", packagePath)

					return m, tea.Sequence(
						copyToClipboardCmd(packagePath),
						tea.Quit,
					)
				}
			}

		case "backspace":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.filterPackages()
			}

		default:
			if len(msg.String()) == 1 {
				m.searchQuery += msg.String()
				m.filterPackages()
			}
		}

	case packagesLoadedMsg:
		m.packages = msg
		m.loading = false
		m.filterPackages()
		return m, nil

	case errMsg:
		m.err = msg
		m.loading = false
		m.quitting = true
		m.finalMessage = fmt.Sprintf("Error: %v", msg)
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.pageSize = msg.Height - 10
		if m.pageSize < 1 {
			m.pageSize = 1
		}
		m.updateViewportOffset()
	}

	return m, nil
}

func (m *model) updateViewportOffset() {
	if m.selectedIndex < m.viewportOffset {
		m.viewportOffset = m.selectedIndex
	} else if m.selectedIndex >= m.viewportOffset+m.pageSize {
		m.viewportOffset = m.selectedIndex - m.pageSize + 1
	}
}

func (m *model) filterPackages() {
	if m.searchQuery == "" {
		m.filtered = make([]fuzzy.Match, len(m.packages))
		for i, p := range m.packages {
			m.filtered[i] = fuzzy.Match{Str: p.Path, Index: i, MatchedIndexes: nil}
		}
	} else {
		targets := make([]string, len(m.packages))
		for i, p := range m.packages {
			targets[i] = p.Path
		}
		m.filtered = fuzzy.Find(m.searchQuery, targets)
	}

	if m.selectedIndex >= len(m.filtered) {
		m.selectedIndex = len(m.filtered) - 1
	}
	if m.selectedIndex < 0 && len(m.filtered) > 0 {
		m.selectedIndex = 0
	}
	m.updateViewportOffset()
}

func (m model) View() string {
	if m.quitting {
		if m.err != nil {
			return errorStyle.Render(m.finalMessage) + "\n"
		}
		return successMessageStyle.Render(m.finalMessage) + "\n"
	}

	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err))
	}

	if m.loading {
		return statusMessageStyle.Render("Loading Go packages from index.golang.org/index... Please wait.")
	}

	s := strings.Builder{}
	s.WriteString(fmt.Sprintf("Search: %s%s\n\n", m.searchQuery, inputStyle.Render("|")))

	if len(m.filtered) == 0 && m.searchQuery != "" {
		s.WriteString("No packages found matching your query.\n")
	} else if len(m.filtered) == 0 && m.searchQuery == "" && !m.loading {
		s.WriteString("No packages loaded.\n")
	} else {
		endIndex := m.viewportOffset + m.pageSize
		if endIndex > len(m.filtered) {
			endIndex = len(m.filtered)
		}

		for i := m.viewportOffset; i < endIndex; i++ {
			item := m.filtered[i]
			pkg := m.packages[item.Index] // Retrieve the full Package struct

			line := item.Str       // This is the package path that fuzzy matched
			version := pkg.Version // Get the version

			var highlightedLine []rune
			lastIndex := 0
			for _, idx := range item.MatchedIndexes {
				highlightedLine = append(highlightedLine, []rune(line[lastIndex:idx])...)
				highlightedLine = append(highlightedLine, []rune(lipgloss.NewStyle().Foreground(lipgloss.Color("#ff00ff")).Render(string(line[idx])))...)
				lastIndex = idx + 1
			}
			highlightedLine = append(highlightedLine, []rune(line[lastIndex:])...)
			displayLine := string(highlightedLine)

			// Append version, styled, if available
			if version != "" {
				displayLine += versionStyle.Render(fmt.Sprintf("(%s)", version))
			}

			if i == m.selectedIndex {
				s.WriteString(selectedItemStyle.Render(displayLine))
			} else {
				s.WriteString(itemStyle.Render(displayLine))
			}
			s.WriteString("\n")
		}
	}

	s.WriteString("\n")
	s.WriteString(statusMessageStyle.Render(fmt.Sprintf("Found %d packages (filtered from %d). Use ↑↓ to navigate, Enter to copy path and quit, Q or Ctrl+C to quit.", len(m.filtered), len(m.packages))))
	return s.String()
}

type packagesLoadedMsg []Package
type errMsg error

func fetchPackagesCmd() tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get("https://index.golang.org/index")
		if err != nil {
			return errMsg(fmt.Errorf("failed to fetch Go index: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return errMsg(fmt.Errorf("received non-OK status from Go index: %s", resp.Status))
		}

		var packages []Package
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			var pkg Package
			if err := json.Unmarshal(line, &pkg); err != nil {
				log.Printf("Error unmarshalling package line: %v, line: %s", err, string(line))
				continue
			}
			packages = append(packages, pkg)
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			return errMsg(fmt.Errorf("error reading Go index response: %w", err))
		}

		return packagesLoadedMsg(packages)
	}
}

func copyToClipboardCmd(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		var cmdName string

		switch runtime.GOOS {
		case "darwin": // macOS
			cmdName = "pbcopy"
			cmd = exec.Command(cmdName)
		case "linux": // Linux
			cmdName = "xclip"
			cmd = exec.Command(cmdName, "-selection", "clipboard", "-i")
		case "windows": // Windows
			cmdName = "clip"
			cmd = exec.Command("cmd", "/c", cmdName)
		default:
			return errMsg(fmt.Errorf("unsupported operating system for clipboard: %s", runtime.GOOS))
		}

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		stdin, err := cmd.StdinPipe()
		if err != nil {
			return errMsg(fmt.Errorf("failed to get stdin pipe for %s: %w", cmdName, err))
		}

		go func() {
			defer stdin.Close()
			_, writeErr := io.WriteString(stdin, text)
			if writeErr != nil {
				log.Printf("Error writing to %s stdin: %v", cmdName, writeErr)
			}
		}()

		if err := cmd.Run(); err != nil {
			errorOutput := strings.TrimSpace(stderr.String())
			detailedErr := fmt.Errorf("failed to copy to clipboard using '%s': %w", cmdName, err)

			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == 127 {
					detailedErr = fmt.Errorf("clipboard command '%s' not found. Please ensure it's installed and in your PATH. (Stderr: %s)", cmdName, errorOutput)
				} else {
					detailedErr = fmt.Errorf("clipboard command '%s' exited with error %d: %w (Stderr: %s)", cmdName, exitErr.ExitCode(), err, errorOutput)
				}
			} else {
				detailedErr = fmt.Errorf("clipboard command '%s' failed: %w (Stderr: %s)", cmdName, err, errorOutput)
			}
			return errMsg(detailedErr)
		}
		return nil
	}
}

func main() {
	m := model{
		loading:  true,
		pageSize: 20,
	}

	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}
