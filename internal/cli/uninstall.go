package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove relith MCP configuration from AI coding agents",
	Long: `Removes the relith MCP server entry from installed AI coding agent configs.

Supports the same agents as 'relith install':
  - OpenCode     ~/.config/opencode/opencode.jsonc
  - Cursor       ~/.cursor/mcp.json
  - Claude Code  ~/.config/claude/mcp.json

Examples:
  relith uninstall             Remove from all detected agents
  relith uninstall --agent=code  Remove from Claude Code only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agentFilter, _ := cmd.Flags().GetString("agent")

		removed := 0

		if agentFilter == "" || agentFilter == "opencode" {
			if err := uninstallOpenCode(); err != nil {
				fmt.Printf("  opencode: %v\n", err)
			} else {
				removed++
			}
		}
		if agentFilter == "" || agentFilter == "cursor" {
			if err := uninstallCursor(); err != nil {
				fmt.Printf("  cursor: %v\n", err)
			} else {
				removed++
			}
		}
		if agentFilter == "" || agentFilter == "code" || agentFilter == "claude" {
			if err := uninstallClaudeCode(); err != nil {
				fmt.Printf("  claude-code: %v\n", err)
			} else {
				removed++
			}
		}

		if removed == 0 {
			fmt.Println("No agents were modified.")
		} else {
			fmt.Printf("Removed relith MCP config from %d agent(s).\n", removed)
		}

		return nil
	},
}

func init() {
	uninstallCmd.Flags().String("agent", "", "Remove from a specific agent only: opencode, cursor, code")
	rootCmd.AddCommand(uninstallCmd)
}

func removeRelithFromMCP(config map[string]any) bool {
	tools, ok := config["tools"].(map[string]any)
	if !ok {
		return false
	}
	mcpServers, ok := tools["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	if _, exists := mcpServers["relith"]; !exists {
		return false
	}
	delete(mcpServers, "relith")
	if len(mcpServers) == 0 {
		delete(tools, "mcpServers")
	}
	if len(tools) == 0 {
		delete(config, "tools")
	}
	return true
}

func removeRelithFromMCPRoot(config map[string]any) bool {
	mcpServers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		return false
	}
	if _, exists := mcpServers["relith"]; !exists {
		return false
	}
	delete(mcpServers, "relith")
	if len(mcpServers) == 0 {
		delete(config, "mcpServers")
	}
	return true
}

func uninstallOpenCode() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	configFile := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
	config, err := readJSON(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  opencode: not configured")
			return nil
		}
		return fmt.Errorf("read: %w", err)
	}

	if !removeRelithFromMCP(config) {
		fmt.Println("  opencode: relith not found in config")
		return nil
	}

	return writeJSON(configFile, config)
}

func uninstallCursor() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	configFile := filepath.Join(home, ".cursor", "mcp.json")
	config, err := readJSON(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  cursor: not configured")
			return nil
		}
		return fmt.Errorf("read: %w", err)
	}

	if !removeRelithFromMCPRoot(config) {
		fmt.Println("  cursor: relith not found in config")
		return nil
	}

	return writeJSON(configFile, config)
}

func uninstallClaudeCode() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	configFile := filepath.Join(home, ".config", "claude", "mcp.json")
	config, err := readJSON(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("  claude-code: not configured")
			return nil
		}
		return fmt.Errorf("read: %w", err)
	}

	if !removeRelithFromMCPRoot(config) {
		fmt.Println("  claude-code: relith not found in config")
		return nil
	}

	return writeJSON(configFile, config)
}

func readJSON(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return config, nil
}

func writeJSON(path string, config map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	fmt.Printf("  removed relith from %s\n", path)
	return nil
}
