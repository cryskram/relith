package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install relith MCP server for AI coding agents",
	Long: `Auto-detects installed AI coding agents and configures them to use
relith's MCP server for code intelligence.

Supported agents:
  - OpenCode     ~/.config/opencode/opencode.jsonc
  - Cursor       ~/.cursor/mcp.json
  - Claude Code  ~/.config/claude/mcp.json

Examples:
  relith install               Auto-detect and configure all supported agents
  relith install --agent=code  Install for Claude Code only`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agentFilter, _ := cmd.Flags().GetString("agent")

		relithBin, err := os.Executable()
		if err != nil {
			return fmt.Errorf("find binary path: %w", err)
		}
		binDir := filepath.Dir(relithBin)
		mcpBin := filepath.Join(binDir, "relithmcp")
		if runtime.GOOS == "windows" {
			mcpBin += ".exe"
		}

		installed := 0

		if agentFilter == "" || agentFilter == "opencode" {
			if err := installOpenCode(mcpBin); err != nil {
				fmt.Printf("  opencode: %v\n", err)
			} else {
				installed++
			}
		}
		if agentFilter == "" || agentFilter == "cursor" {
			if err := installCursor(mcpBin); err != nil {
				fmt.Printf("  cursor: %v\n", err)
			} else {
				installed++
			}
		}
		if agentFilter == "" || agentFilter == "code" || agentFilter == "claude" {
			if err := installClaudeCode(mcpBin); err != nil {
				fmt.Printf("  claude-code: %v\n", err)
			} else {
				installed++
			}
		}

		if installed == 0 {
			fmt.Println("No supported agents detected. Install one of:")
			fmt.Println("  - OpenCode:  https://github.com/anomalyco/opencode")
			fmt.Println("  - Cursor:    https://cursor.sh")
			fmt.Println("  - Claude Code: https://claude.ai/code")
			fmt.Println()
			fmt.Println("Or use --agent flag to target a specific agent:")
			fmt.Println("  relith install --agent=opencode")
			fmt.Println("  relith install --agent=cursor")
			fmt.Println("  relith install --agent=code")
		} else {
			fmt.Printf("Configured %d agent(s) to use relith MCP server.\n", installed)
			fmt.Println("Restart your agent for the changes to take effect.")
		}

		return nil
	},
}

func init() {
	installCmd.Flags().String("agent", "", "Install for a specific agent only: opencode, cursor, code")
	rootCmd.AddCommand(installCmd)
}

func installOpenCode(mcpBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	configDir := filepath.Join(home, ".config", "opencode")
	configFile := filepath.Join(configDir, "opencode.jsonc")
	relithmcpCmd := mcpBin

	config := map[string]any{
		"tools": map[string]any{
			"mcpServers": map[string]any{
				"relith": map[string]any{
					"command": relithmcpCmd,
					"args":    []string{},
				},
			},
		},
	}

	if _, err := os.Stat(configFile); err == nil {
		existing, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("read %s: %w", configFile, err)
		}

		var existingMap map[string]any
		if err := json.Unmarshal(existing, &existingMap); err == nil {
			if tools, ok := existingMap["tools"].(map[string]any); ok {
				if mcpMap, ok := tools["mcpServers"].(map[string]any); ok {
					if _, has := mcpMap["relith"]; has {
						fmt.Println("  opencode: already configured (edit opencode.jsonc to update)")
						return nil
					}
				}
			}
		}

		// Merge: add relith to existing mcpServers
		if tools, ok := existingMap["tools"].(map[string]any); ok {
			if mcpServers, ok := tools["mcpServers"].(map[string]any); ok {
				mcpServers["relith"] = map[string]any{
					"command": relithmcpCmd,
					"args":    []string{},
				}
			} else {
				tools["mcpServers"] = map[string]any{
					"relith": map[string]any{
						"command": relithmcpCmd,
						"args":    []string{},
					},
				}
			}
		} else {
			existingMap["tools"] = map[string]any{
				"mcpServers": map[string]any{
					"relith": map[string]any{
						"command": relithmcpCmd,
						"args":    []string{},
					},
				},
			}
		}
		config = existingMap
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", configFile, err)
	}

	fmt.Printf("  opencode: configured at %s\n", configFile)
	return nil
}

func installCursor(mcpBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	configDir := filepath.Join(home, ".cursor")
	configFile := filepath.Join(configDir, "mcp.json")

	mcpConfig := map[string]any{
		"relith": map[string]any{
			"command": mcpBin,
			"args":    []string{},
		},
	}

	var final map[string]any
	if existing, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(existing, &final); err == nil {
			if mcpServers, ok := final["mcpServers"].(map[string]any); ok {
				if _, has := mcpServers["relith"]; has {
					fmt.Println("  cursor: already configured (edit mcp.json to update)")
					return nil
				}
				mcpServers["relith"] = mcpConfig["relith"]
			} else {
				final["mcpServers"] = mcpConfig
			}
		}
	}
	if final == nil {
		final = map[string]any{
			"mcpServers": mcpConfig,
		}
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(final, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", configFile, err)
	}

	fmt.Printf("  cursor: configured at %s\n", configFile)
	return nil
}

func installClaudeCode(mcpBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}

	configDir := filepath.Join(home, ".config", "claude")
	configFile := filepath.Join(configDir, "mcp.json")

	mcpConfig := map[string]any{
		"relith": map[string]any{
			"command": mcpBin,
			"args":    []string{},
		},
	}

	var final map[string]any
	if existing, err := os.ReadFile(configFile); err == nil {
		if err := json.Unmarshal(existing, &final); err == nil {
			if mcpServers, ok := final["mcpServers"].(map[string]any); ok {
				if _, has := mcpServers["relith"]; has {
					fmt.Println("  claude-code: already configured (edit mcp.json to update)")
					return nil
				}
				mcpServers["relith"] = mcpConfig["relith"]
			} else {
				final["mcpServers"] = mcpConfig
			}
		}
	}
	if final == nil {
		final = map[string]any{
			"mcpServers": mcpConfig,
		}
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	data, err := json.MarshalIndent(final, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("write %s: %w", configFile, err)
	}

	fmt.Printf("  claude-code: configured at %s\n", configFile)
	return nil
}
