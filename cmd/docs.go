package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/poldebatecli/internal/docs"
	"github.com/spf13/cobra"
)

var docsCmd = &cobra.Command{
	Use:   "docs [topic]",
	Short: "Print project documentation to stdout",
	Long: `Print project documentation to stdout or open in a browser.

Topics: architecture, configuration, cli, prompts, failure-modes, all`,
	Example: `  debate docs architecture
  debate docs configuration
  debate docs all
  debate docs --open cli`,
	RunE: runDocs,
	Args: cobra.MaximumNArgs(1),
}

var docsOpen bool

func init() {
	rootCmd.AddCommand(docsCmd)
	docsCmd.Flags().BoolVar(&docsOpen, "open", false, "Open documentation in default browser instead of printing")
}

func runDocs(cmd *cobra.Command, args []string) error {
	topic := "all"
	if len(args) > 0 {
		topic = args[0]
	}

	var content string
	if topic == "all" {
		content = docs.AllContent()
	} else {
		var err error
		content, err = docs.GetSection(topic)
		if err != nil {
			available := docs.ListSections()
			return fmt.Errorf("unknown topic '%s'. Available: %v", topic, available)
		}
	}

	if docsOpen {
		return openInBrowser(content, topic)
	}

	fmt.Print(content)
	return nil
}

func openInBrowser(content, topic string) error {
	tmpDir := os.TempDir()
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	filename := fmt.Sprintf("poldebatecli-docs-%s-%s.md", topic, hex.EncodeToString(randBytes))
	tmpFile := filepath.Join(tmpDir, filename)
	if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", tmpFile)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", tmpFile)
	default:
		cmd = exec.Command("xdg-open", tmpFile)
	}

	if err := cmd.Start(); err != nil {
		fmt.Print(content)
		fmt.Fprintf(os.Stderr, "\nCould not open browser: %v\nContent printed to stdout instead.\n", err)
	}

	return nil
}
