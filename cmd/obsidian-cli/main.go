package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	defaultServerURL = "http://localhost:8080"
	defaultVaultID   = "default"
)

type Config struct {
	ServerURL string
	VaultID   string
}

func main() {
	// Define subcommands
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)

	listCmd := flag.NewFlagSet("list", flag.ExitOnError)

	searchCmd := flag.NewFlagSet("search", flag.ExitOnError)
	searchType := searchCmd.String("type", "text", "Search type: text, tag, wikilink, fuzzy, phrase, prefix, title")
	searchTags := searchCmd.String("tags", "", "Comma-separated tags for tag search")
	searchWikis := searchCmd.String("wikilinks", "", "Comma-separated wikilinks for wikilink search")
	searchLimit := searchCmd.Int("limit", 50, "Max results")

	viewCmd := flag.NewFlagSet("view", flag.ExitOnError)

	// Global flags (handled manually or passed to subcommands if we used a library,
	// but with standard flag, we'll parse them from env or a helper)
	// For simplicity, we'll use env vars for global config or flags on the subcommands if needed.
	// Let's add common flags to all subcommands for server/vault
	cmds := []*flag.FlagSet{statusCmd, listCmd, searchCmd, viewCmd}
	configs := make([]*Config, len(cmds))

	for i, cmd := range cmds {
		configs[i] = &Config{}
		cmd.StringVar(&configs[i].ServerURL, "server", defaultServerURL, "Server URL")
		if cmd != statusCmd { // status lists all vaults, usually doesn't need a specific one, but consistent flags help
			cmd.StringVar(&configs[i].VaultID, "vault", defaultVaultID, "Vault ID")
		}
	}

	if len(os.Args) < 2 {
		fmt.Println("Expected 'status', 'list', 'search', or 'view' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "status":
		statusCmd.Parse(os.Args[2:])
		handleStatus(configs[0])
	case "list":
		listCmd.Parse(os.Args[2:])
		handleList(configs[1])
	case "search":
		searchCmd.Parse(os.Args[2:])
		handleSearch(configs[2], searchCmd.Args(), *searchType, *searchTags, *searchWikis, *searchLimit)
	case "view":
		viewCmd.Parse(os.Args[2:])
		if viewCmd.NArg() < 1 {
			fmt.Println("Error: file ID required")
			os.Exit(1)
		}
		handleView(configs[3], viewCmd.Arg(0))
	default:
		fmt.Println("Expected 'status', 'list', 'search', or 'view' subcommands")
		os.Exit(1)
	}
}

// --- Handlers ---

func handleStatus(cfg *Config) {
	url := fmt.Sprintf("%s/api/v1/vaults", cfg.ServerURL)
	resp, err := http.Get(url)
	if err != nil {
		fatal("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fatal("Server returned status: %s", resp.Status)
	}

	var result struct {
		Vaults []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
			Active bool   `json:"active"`
		} `json:"vaults"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("Failed to decode response: %v", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tACTIVE")
	for _, v := range result.Vaults {
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", v.ID, v.Name, v.Status, v.Active)
	}
	w.Flush()
}

func handleList(cfg *Config) {
	// Using the /api/v1/files/tree/{vault} endpoint
	url := fmt.Sprintf("%s/api/v1/files/tree/%s", cfg.ServerURL, cfg.VaultID)
	resp, err := http.Get(url)
	if err != nil {
		fatal("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fatal("Server returned status: %s", resp.Status)
	}

	// We'll decode to a map to handle the recursive structure more flexibly or just pretty print
	// But to list files, a recursive print function is better.
	var rawResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResult); err != nil {
		fatal("Failed to decode response: %v", err)
	}

	nodes, ok := rawResult["nodes"].([]interface{})
	if !ok {
		fatal("Invalid response format")
	}

	fmt.Printf("Files in vault '%s':\n", cfg.VaultID)
	printTree(nodes, "")
}

func printTree(nodes []interface{}, prefix string) {
	for i, n := range nodes {
		node, ok := n.(map[string]interface{})
		if !ok {
			continue
		}

		meta, ok := node["metadata"].(map[string]interface{})
		if !ok {
			continue // Should not happen based on API
		}

		name, _ := meta["name"].(string)
		id, _ := meta["id"].(string)
		isDir, _ := meta["is_directory"].(bool)

		isLast := i == len(nodes)-1
		marker := "├──"
		if isLast {
			marker = "└──"
		}

		fmt.Printf("%s%s %s (ID: %s)\n", prefix, marker, name, id)

		if isDir {
			children, ok := node["children"].([]interface{})
			if ok && len(children) > 0 {
				newPrefix := prefix + "│   "
				if isLast {
					newPrefix = prefix + "    "
				}
				printTree(children, newPrefix)
			}
		}
	}
}

func handleSearch(cfg *Config, args []string, searchType, tags, wikilinks string, limit int) {
	query := strings.Join(args, " ")

	// Construct request body
	reqBody := map[string]interface{}{
		"query":      query,
		"type":       searchType,
		"limit":      limit,
		"title_only": false,
	}

	if tags != "" {
		reqBody["tags"] = strings.Split(tags, ",")
	}
	if wikilinks != "" {
		reqBody["wikilinks"] = strings.Split(wikilinks, ",")
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		fatal("Failed to marshal request: %v", err)
	}

	url := fmt.Sprintf("%s/api/v1/search/%s", cfg.ServerURL, cfg.VaultID)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fatal("Failed to perform search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fatal("Server returned error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Total   int `json:"total"`
		Results []struct {
			ID        string                 `json:"id"`
			Score     float64                `json:"score"`
			Fields    map[string]interface{} `json:"fields"`
			Fragments map[string][]string    `json:"fragments"`
		} `json:"results"`
		Took string `json:"took"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("Failed to decode response: %v", err)
	}

	fmt.Printf("Found %d results in %s:\n\n", result.Total, result.Took)

	for _, res := range result.Results {
		title := res.ID // Default to ID
		if t, ok := res.Fields["title"].(string); ok && t != "" {
			title = t
		} else if path, ok := res.Fields["path"].(string); ok {
			title = path
		}

		fmt.Printf("[%s] %s (Score: %.2f)\n", res.ID, title, res.Score)

		// Print snippets/fragments if available
		if len(res.Fragments) > 0 {
			for field, frags := range res.Fragments {
				for _, frag := range frags {
					fmt.Printf("  %s: ...%s...\n", field, frag)
				}
			}
			fmt.Println()
		}
	}
}

func handleView(cfg *Config, fileID string) {
	url := fmt.Sprintf("%s/api/v1/files/by-id/%s/%s", cfg.ServerURL, cfg.VaultID, fileID)
	resp, err := http.Get(url)
	if err != nil {
		fatal("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fatal("Server returned error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Content string `json:"content"`
		Path    string `json:"path"`
		Name    string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fatal("Failed to decode response: %v", err)
	}

	fmt.Printf("--- %s (%s) ---\n\n", result.Name, result.Path)
	fmt.Println(result.Content)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
