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
	baseURL = "https://app.terraform.io/api/v2"
)

type Workspace struct {
	ID         string `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
}

type WorkspacesResponse struct {
	Data  []Workspace `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

type Variable struct {
	ID         string `json:"id"`
	Attributes struct {
		Key       string `json:"key"`
		Value     string `json:"value"`
		Sensitive bool   `json:"sensitive"`
		Category  string `json:"category"`
		HCL       bool   `json:"hcl"`
	} `json:"attributes"`
}

type VariablesResponse struct {
	Data []Variable `json:"data"`
}

func newRequest(token, method, url string) (*http.Request, error) {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	return req, nil
}

func fetchWorkspaces(token, org string) ([]Workspace, error) {
	client := &http.Client{}
	var all []Workspace
	url := fmt.Sprintf("%s/organizations/%s/workspaces?page[size]=100", baseURL, org)

	for url != "" {
		req, err := newRequest(token, "GET", url)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
		}

		var result WorkspacesResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}

		all = append(all, result.Data...)
		url = result.Links.Next
	}

	return all, nil
}

func fetchVariables(token, workspaceID string) ([]Variable, error) {
	client := &http.Client{}
	url := fmt.Sprintf("%s/workspaces/%s/vars", baseURL, workspaceID)

	req, err := newRequest(token, "GET", url)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var result VariablesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Data, nil
}

func resolveWorkspaceID(workspaces []Workspace, name string) (string, error) {
	for _, ws := range workspaces {
		if ws.Attributes.Name == name {
			return ws.ID, nil
		}
	}
	return "", fmt.Errorf("workspace %q not found", name)
}

func createVariable(token, workspaceID string, v Variable) error {
	body := map[string]interface{}{
		"data": map[string]interface{}{
			"type": "vars",
			"attributes": map[string]interface{}{
				"key":       v.Attributes.Key,
				"value":     v.Attributes.Value,
				"sensitive": v.Attributes.Sensitive,
				"category":  v.Attributes.Category,
				"hcl":       v.Attributes.HCL,
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/workspaces/%s/vars", baseURL, workspaceID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/vnd.api+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func updateVariable(token, workspaceID, varID string, v Variable) error {
	body := map[string]interface{}{
		"data": map[string]interface{}{
			"id":   varID,
			"type": "vars",
			"attributes": map[string]interface{}{
				"key":       v.Attributes.Key,
				"value":     v.Attributes.Value,
				"sensitive": v.Attributes.Sensitive,
				"category":  v.Attributes.Category,
				"hcl":       v.Attributes.HCL,
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/workspaces/%s/vars/%s", baseURL, workspaceID, varID)
	req, err := http.NewRequest("PATCH", url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/vnd.api+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func cmdCopyVars(token, org, fromName, toName string, overwrite, copySensitive bool) {
	workspaces, err := fetchWorkspaces(token, org)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fromID, err := resolveWorkspaceID(workspaces, fromName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: source: %v\n", err)
		os.Exit(1)
	}

	toID, err := resolveWorkspaceID(workspaces, toName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: destination: %v\n", err)
		os.Exit(1)
	}

	srcVars, err := fetchVariables(token, fromID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching source variables: %v\n", err)
		os.Exit(1)
	}

	dstVars, err := fetchVariables(token, toID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching destination variables: %v\n", err)
		os.Exit(1)
	}

	// Build lookup of existing destination vars: key -> variable
	existing := make(map[string]Variable)
	for _, v := range dstVars {
		existing[v.Attributes.Key] = v
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Copying variables from %q → %q\n\n", fromName, toName)
	fmt.Fprintln(w, "KEY\tACTION\tNOTE")
	fmt.Fprintln(w, "---\t------\t----")

	for _, v := range srcVars {
		if v.Attributes.Sensitive {
			if !copySensitive {
				fmt.Fprintf(w, "%s\tskipped\tsensitive (use --copy-sensitive to copy with empty value)\n", v.Attributes.Key)
				continue
			}
			v.Attributes.Value = ""
		}

		if dst, exists := existing[v.Attributes.Key]; exists {
			if !overwrite {
				fmt.Fprintf(w, "%s\tskipped\talready exists (use --overwrite to replace)\n", v.Attributes.Key)
				continue
			}
			if err := updateVariable(token, toID, dst.ID, v); err != nil {
				fmt.Fprintf(w, "%s\tfailed\t%v\n", v.Attributes.Key, err)
			} else {
				fmt.Fprintf(w, "%s\tupdated\t\n", v.Attributes.Key)
			}
		} else {
			if err := createVariable(token, toID, v); err != nil {
				fmt.Fprintf(w, "%s\tfailed\t%v\n", v.Attributes.Key, err)
			} else {
				fmt.Fprintf(w, "%s\tcreated\t\n", v.Attributes.Key)
			}
		}
	}

	w.Flush()
}

func cmdListWorkspaces(token, org, filter string) {
	workspaces, err := fetchWorkspaces(token, org)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME")
	fmt.Fprintln(w, "--\t----")
	for _, ws := range workspaces {
		if filter != "" && !strings.Contains(ws.Attributes.Name, filter) {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\n", ws.ID, ws.Attributes.Name)
	}
	w.Flush()
}

func cmdListVars(token, org, workspaceName string) {
	workspaces, err := fetchWorkspaces(token, org)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var wsID string
	for _, ws := range workspaces {
		if ws.Attributes.Name == workspaceName {
			wsID = ws.ID
			break
		}
	}

	if wsID == "" {
		fmt.Fprintf(os.Stderr, "error: workspace %q not found\n", workspaceName)
		os.Exit(1)
	}

	vars, err := fetchVariables(token, wsID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(vars) == 0 {
		fmt.Println("No variables found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE\tCATEGORY\tSENSITIVE")
	fmt.Fprintln(w, "---\t-----\t--------\t---------")
	for _, v := range vars {
		val := v.Attributes.Value
		if v.Attributes.Sensitive {
			val = "***"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%v\n",
			v.Attributes.Key,
			val,
			v.Attributes.Category,
			v.Attributes.Sensitive,
		)
	}
	w.Flush()
}

func usage() {
	fmt.Fprint(os.Stderr, `tfcopyvars - Terraform Cloud workspace variable CLI

Usage:
  tfcopyvars workspaces [--filter <name>]
  tfcopyvars vars --workspace <name>
  tfcopyvars copy-vars --from <name> --to <name> [--overwrite]

Environment:
  TFE_TOKEN   HCP Terraform API token (required)
  TFE_ORG     HCP Terraform organization name (required)

Commands:
  workspaces  List all workspaces
  vars        List variables for a workspace
  copy-vars   Copy variables from one workspace to another

Flags:
`)
	flag.PrintDefaults()
}

func main() {
	token := os.Getenv("TFE_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "error: TFE_TOKEN environment variable is not set")
		fmt.Fprintln(os.Stderr)
		usage()
		os.Exit(1)
	}

	org := os.Getenv("TFE_ORG")
	if org == "" {
		fmt.Fprintln(os.Stderr, "error: TFE_ORG environment variable is not set")
		fmt.Fprintln(os.Stderr)
		usage()
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "workspaces":
		fs := flag.NewFlagSet("workspaces", flag.ExitOnError)
		filter := fs.String("filter", "", "filter workspaces by name substring")
		fs.Parse(os.Args[2:])
		cmdListWorkspaces(token, org, *filter)

	case "vars":
		fs := flag.NewFlagSet("vars", flag.ExitOnError)
		workspace := fs.String("workspace", "", "workspace name (required)")
		fs.Parse(os.Args[2:])
		if *workspace == "" {
			fmt.Fprintln(os.Stderr, "error: --workspace is required")
			fs.Usage()
			os.Exit(1)
		}
		cmdListVars(token, org, *workspace)

	case "copy-vars":
		fs := flag.NewFlagSet("copy-vars", flag.ExitOnError)
		from := fs.String("from", "", "source workspace name (required)")
		to := fs.String("to", "", "destination workspace name (required)")
		overwrite := fs.Bool("overwrite", false, "overwrite existing variables in destination")
		copySensitive := fs.Bool("copy-sensitive", false, "copy sensitive variables with empty value (you must fill them in manually)")
		fs.Parse(os.Args[2:])
		if *from == "" || *to == "" {
			fmt.Fprintln(os.Stderr, "error: --from and --to are required")
			fs.Usage()
			os.Exit(1)
		}
		cmdCopyVars(token, org, *from, *to, *overwrite, *copySensitive)

	default:
		usage()
		os.Exit(1)
	}
}
