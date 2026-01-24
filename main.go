package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"text/tabwriter"
	"time"
)

type Config struct {
	BaseURL     string
	APIKey      string
	ProjectName string
	Version     string
	SbomFile    string
}

type Project struct {
	UUID          string `json:"uuid"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Active        bool   `json:"active"`
	IsLatest      bool   `json:"isLatest"`
	LastBomImport int64  `json:"lastBomImport"` // Epoch millis
}

var config Config

func main() {

	uploadPtr := flag.Bool("upload", false, "Upload the SBOM file")
	listPtr := flag.Bool("list", false, "Display a table of existing versions")
	latestPtr := flag.Bool("latest", false, "Mark the target version as Latest")
	cleanPtr := flag.Bool("clean", false, "Mark all OTHER versions as Inactive")
	ciPtr := flag.Bool("ci", false, "Run full CI mode (Upload + Latest + Clean)")

	urlPtr := flag.String("url", "", "Base URL of Dependency-Track")
	filePtr := flag.String("file", "sbom.json", "Path to the SBOM file")

	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		printUsage()
		os.Exit(1)
	}

	config = Config{
		BaseURL:     *urlPtr,
		SbomFile:    *filePtr,
		APIKey:      args[0],
		ProjectName: args[1],
	}

	if len(args) >= 3 {
		config.Version = args[2]
	}

	if *ciPtr {
		*uploadPtr = true
		*latestPtr = true
		*cleanPtr = true
	}

	actionsRequiringVersion := *uploadPtr || *latestPtr || *cleanPtr
	if actionsRequiringVersion && config.Version == "" {
		fmt.Println("Error: You must provide a [VERSION] argument for -upload, -latest, or -clean.")
		printUsage()
		os.Exit(1)
	}

	if *uploadPtr {
		if err := uploadSBOM(); err != nil {
			fmt.Printf("Error uploading SBOM: %v\n", err)
			os.Exit(1)
		}
	}

	if *listPtr || *latestPtr || *cleanPtr {
		versions, err := getAllVersions()
		if err != nil {
			fmt.Printf("Error fetching versions: %v\n", err)
			os.Exit(1)
		}

		if *listPtr {
			displayVersions(versions)
		}

		if *latestPtr || *cleanPtr {
			updateLifecycle(versions, *latestPtr, *cleanPtr)
		}
	}
}

func printUsage() {
	fmt.Println("Usage: go run main.go [flags] <API_KEY> <PROJECT_NAME> [VERSION]")
	fmt.Println("\nArguments:")
	fmt.Println("  VERSION is optional for -list, but required for -upload/-latest/-clean")
	fmt.Println("\nFlags:")
	flag.PrintDefaults()
}

func uploadSBOM() error {
	fmt.Printf("Uploading SBOM from %s for %s %s...\n", config.SbomFile, config.ProjectName, config.Version)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("autoCreate", "true")
	_ = writer.WriteField("projectName", config.ProjectName)
	_ = writer.WriteField("projectVersion", config.Version)

	file, err := os.Open(config.SbomFile)
	if err != nil {
		return fmt.Errorf("could not open file %s: %v", config.SbomFile, err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("bom", filepath.Base(config.SbomFile))
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", config.BaseURL+"/api/v1/bom", body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", config.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return doRequest(req)
}

func getAllVersions() ([]Project, error) {
	fmt.Println("Fetching all project versions...")
	var allProjects []Project
	page := 1
	pageSize := 50
	client := &http.Client{Timeout: 20 * time.Second}

	for {
		req, err := http.NewRequest("GET", config.BaseURL+"/api/v1/project", nil)
		if err != nil {
			return nil, err
		}

		q := req.URL.Query()
		q.Add("name", config.ProjectName)
		q.Add("excludeInactive", "false")
		q.Add("pageNumber", fmt.Sprintf("%d", page))
		q.Add("pageSize", fmt.Sprintf("%d", pageSize))
		req.URL.RawQuery = q.Encode()

		req.Header.Set("X-Api-Key", config.APIKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("api returned status: %s", resp.Status)
		}

		var pageProjects []Project
		if err := json.NewDecoder(resp.Body).Decode(&pageProjects); err != nil {
			return nil, err
		}

		if len(pageProjects) == 0 {
			break
		}
		allProjects = append(allProjects, pageProjects...)
		if len(pageProjects) < pageSize {
			break
		}
		page++
	}
	slices.SortFunc(allProjects, func(a, b Project) int { return cmp.Compare(a.LastBomImport, b.LastBomImport) })
	return allProjects, nil
}

func displayVersions(projects []Project) {
	fmt.Println("\n--- Current Versions ---")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VERSION\tACTIVE\tLATEST\tLAST UPLOAD\tUUID")

	for _, p := range projects {
		if p.Name != config.ProjectName {
			continue
		}

		ts := "-"
		if p.LastBomImport > 0 {
			t := time.UnixMilli(p.LastBomImport)
			ts = t.Format("2006-01-02 15:04")
		}

		fmt.Fprintf(w, "%s\t%v\t%v\t%s\t%s\n", p.Version, p.Active, p.IsLatest, ts, p.UUID)
	}
	w.Flush()
	fmt.Println("------------------------")
}

func updateLifecycle(projects []Project, updateLatest bool, cleanInactive bool) {
	fmt.Println("Updating Lifecycle...")
	for _, p := range projects {
		if p.Name != config.ProjectName {
			continue
		}

		payload := make(map[string]interface{})
		shouldPatch := false
		isTargetVersion := p.Version == config.Version

		if updateLatest {
			if isTargetVersion {
				if !p.IsLatest {
					payload["isLatest"] = true
					shouldPatch = true
				}
			} else {
				if p.IsLatest {
					payload["isLatest"] = false
					shouldPatch = true
				}
			}
		}

		if cleanInactive {
			if isTargetVersion {
				if !p.Active {
					payload["active"] = true
					shouldPatch = true
				}
			} else {
				if p.Active {
					payload["active"] = false
					shouldPatch = true
				}
			}
		}

		if shouldPatch {
			fmt.Printf(" -> Patching %s: %v\n", p.Version, payload)
			if err := sendPatch(p.UUID, payload); err != nil {
				fmt.Printf("    Error: %v\n", err)
			}
		}
	}
}

func sendPatch(uuid string, payload map[string]interface{}) error {
	jsonBody, _ := json.Marshal(payload)
	req, err := http.NewRequest("PATCH", fmt.Sprintf("%s/api/v1/project/%s", config.BaseURL, uuid), bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", config.APIKey)
	req.Header.Set("Content-Type", "application/json")
	return doRequest(req)
}

func doRequest(req *http.Request) error {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status %s: %s", resp.Status, string(bodyBytes))
	}
	return nil
}
