//go:build mage

// Sort viewer tasks. Run from services/sortviewer: mage build, mage test, mage deploy,
// mage keeperSetup (once).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/magefile/mage/sh"
)

const (
	appName    = "sortviewer"
	keeperApp  = "SortViewer"
	keeperEnv  = "production"
	domain     = "lego.n8p.xyz"
	appURL     = "https://" + domain
	buildDir   = "build"
	embedDir   = "backend/webui/dist"
	uploadFile = buildDir + "/upload/" + appName // Dry Dock names versions after the file
)

// adminEmails may sign in as admin (through the Gangway). ADMIN_EMAILS overrides it.
func adminEmails() string {
	if v := os.Getenv("ADMIN_EMAILS"); v != "" {
		return v
	}
	return "n8utah@gmail.com"
}

// Build makes build/sortviewer-linux: one linux/amd64 binary with the web pages inside.
func Build() error {
	if err := BuildFrontend(); err != nil {
		return err
	}
	if err := embedFrontend(); err != nil {
		return err
	}
	return BuildBackend()
}

// BuildFrontend installs the frontend's packages and builds it into frontend/dist.
func BuildFrontend() error {
	npm := "npm"
	if runtime.GOOS == "windows" {
		npm = "npm.cmd"
	}
	if err := sh.RunV(npm, "--prefix", "frontend", "ci", "--no-audit", "--no-fund"); err != nil {
		if err := sh.RunV(npm, "--prefix", "frontend", "install", "--no-audit", "--no-fund"); err != nil {
			return err
		}
	}
	return sh.RunV(npm, "--prefix", "frontend", "run", "build")
}

// BuildBackend cross-compiles the server for linux, embedding backend/webui/dist.
func BuildBackend() error {
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	env := map[string]string{"GOOS": "linux", "GOARCH": "amd64", "CGO_ENABLED": "0"}
	return sh.RunWithV(env, "go", "build", "-trimpath",
		"-ldflags", "-s -w -X main.commit="+gitCommit(),
		"-o", filepath.Join(buildDir, appName+"-linux"), "./backend")
}

// Test runs the Go tests and lints the frontend.
func Test() error {
	if err := sh.RunV("go", "test", "./..."); err != nil {
		return err
	}
	npx := "npx"
	if runtime.GOOS == "windows" {
		npx = "npx.cmd"
	}
	return sh.RunV(npx, "--prefix", "frontend", "eslint", "frontend")
}

// Deploy builds and ships to Harbor Keeper (keeper CLI, logged in), then checks the
// live site runs the new build.
func Deploy() error {
	keeper, err := keeperCLI()
	if err != nil {
		return err
	}
	if err := Build(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(uploadFile), 0o755); err != nil {
		return err
	}
	if err := sh.Copy(filepath.FromSlash(uploadFile), filepath.Join(buildDir, appName+"-linux")); err != nil {
		return err
	}
	fmt.Printf("Deploying to %s / %s...\n", keeperApp, keeperEnv)
	if err := sh.RunV(keeper, "push", keeperApp, keeperEnv, "-file", filepath.FromSlash(uploadFile), "-activate"); err != nil {
		return fmt.Errorf("keeper push failed: %w", err)
	}
	want := gitCommit()
	var last string
	for i := 0; i < 20; i++ {
		var health struct {
			Commit string `json:"commit"`
		}
		if err := getJSON(appURL+"/api/health", &health); err == nil && health.Commit == want {
			fmt.Printf("Deployed. %s runs %s.\n", appURL, want)
			return nil
		} else if err != nil {
			last = err.Error()
		} else {
			last = "it reports " + health.Commit
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("deployed, but %s doesn't report commit %s (%s); see: keeper logs %s %s", appURL, want, last, keeperApp, keeperEnv)
}

// KeeperSetup makes the Dry Dock app (always on) at lego.n8p.xyz, registers its
// Gangway OIDC client, gives the admins access, and sets its settings. Safe to run
// again: it only adds what's missing (an existing client keeps its secret).
func KeeperSetup() error {
	keeper, err := keeperCLI()
	if err != nil {
		return err
	}
	serviceID, err := keeperServiceID(keeper)
	if err != nil {
		return err
	}
	if serviceID == "" {
		fmt.Printf("Creating %s at %s...\n", keeperApp, domain)
		if err := sh.RunV(keeper, "create", keeperApp, "-domain", domain, "-idle", "always"); err != nil {
			return err
		}
		if serviceID, err = keeperServiceID(keeper); err != nil || serviceID == "" {
			return fmt.Errorf("created %s, but can't find it in keeper apps (%v)", keeperApp, err)
		}
	} else {
		fmt.Printf("%s exists (%s); making sure it's always on.\n", keeperApp, serviceID)
		if err := sh.RunV(keeper, "idle", keeperApp, "always"); err != nil {
			return err
		}
	}

	api, err := newKeeperAPI()
	if err != nil {
		return err
	}
	var apps struct {
		Apps []struct {
			ServiceID string `json:"serviceId"`
			ClientID  string `json:"clientId"`
		} `json:"apps"`
		Issuer string `json:"issuer"`
	}
	if err := api.do("GET", "/gangway/apps", nil, &apps); err != nil {
		return fmt.Errorf("read the Gangway's apps: %w", err)
	}
	if apps.Issuer == "" {
		return fmt.Errorf("the Gangway has no login domain set")
	}
	clientID := ""
	for _, a := range apps.Apps {
		if a.ServiceID == serviceID {
			clientID = a.ClientID
		}
	}
	change := map[string]any{
		"gate": false, "allowSignup": false, "showInPortal": true, "portalUrl": appURL + "/auth/login",
		"publicClient": false, "redirectUris": []string{appURL + "/auth/callback"},
		"postLogoutUris": []string{appURL + "/"}, "backchannelLogoutUri": appURL + "/auth/backchannel-logout",
	}
	if err := api.do("PUT", "/gangway/apps/"+serviceID, change, nil); err != nil {
		return fmt.Errorf("set up sign-in for %s: %w", keeperApp, err)
	}
	if clientID == "" {
		var made struct {
			Client struct {
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
			} `json:"client"`
		}
		if err := api.do("POST", "/gangway/apps/"+serviceID+"/client", nil, &made); err != nil {
			return fmt.Errorf("make the OIDC client: %w", err)
		}
		clientID = made.Client.ClientID
		if err := sh.RunV(keeper, "vars", "set", keeperApp, keeperEnv,
			"OIDC_CLIENT_SECRET="+made.Client.ClientSecret, "-secret"); err != nil {
			return err
		}
		fmt.Println("Made the OIDC client; its secret is set as a secret.")
	} else {
		fmt.Println("The OIDC client exists; keeping its secret.")
	}

	if err := grantAccess(api, serviceID); err != nil {
		return err
	}
	return sh.RunV(keeper, "vars", "set", keeperApp, keeperEnv,
		"APP_URL="+appURL, "OIDC_ISSUER="+apps.Issuer, "OIDC_CLIENT_ID="+clientID,
		"ADMIN_EMAILS="+adminEmails(), "-plain")
}

// grantAccess gives each admin's Gangway user access to the app (admins of the
// Gangway may use every app already).
func grantAccess(api *keeperAPI, serviceID string) error {
	var users struct {
		Users []struct {
			ID         string   `json:"id"`
			Email      string   `json:"email"`
			Admin      bool     `json:"admin"`
			ServiceIDs []string `json:"serviceIds"`
		} `json:"users"`
	}
	if err := api.do("GET", "/gangway/users", nil, &users); err != nil {
		return fmt.Errorf("read the Gangway's users: %w", err)
	}
	for _, email := range strings.Split(adminEmails(), ",") {
		email = strings.TrimSpace(email)
		found := false
		for _, u := range users.Users {
			if !strings.EqualFold(u.Email, email) {
				continue
			}
			found = true
			if u.Admin || slices.Contains(u.ServiceIDs, serviceID) {
				fmt.Printf("%s can already use %s.\n", email, keeperApp)
				break
			}
			body := map[string]any{"serviceIds": append(slices.Clone(u.ServiceIDs), serviceID)}
			if err := api.do("PUT", "/gangway/users/"+u.ID+"/access", body, nil); err != nil {
				return fmt.Errorf("give %s access: %w", email, err)
			}
			fmt.Printf("Gave %s access to %s.\n", email, keeperApp)
		}
		if !found {
			fmt.Printf("Warning: %s isn't a Gangway user, so can't sign in as admin yet.\n", email)
		}
	}
	return nil
}

func keeperCLI() (string, error) {
	keeper, err := exec.LookPath("keeper")
	if err != nil {
		return "", fmt.Errorf("keeper CLI not found; install it with: go install github.com/n8utah/Harborkeeper/cmd/keeper@latest")
	}
	if _, err := sh.Output(keeper, "apps"); err != nil {
		return "", fmt.Errorf("keeper can't reach Harbor Keeper (%v); log in with: keeper login -url https://n8p.xyz", err)
	}
	return keeper, nil
}

// keeperServiceID is the app's id from keeper apps ("" if it isn't there).
func keeperServiceID(keeper string) (string, error) {
	out, err := sh.Output(keeper, "apps")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\t")
		if len(fields) >= 2 && fields[1] == keeperApp {
			return fields[0], nil
		}
	}
	return "", nil
}

// keeperAPI calls Harbor Keeper's admin API with the keeper CLI's saved login, for what
// the CLI doesn't cover yet (the Gangway).
type keeperAPI struct {
	url, token string
}

func newKeeperAPI() (*keeperAPI, error) {
	if u, t := os.Getenv("KEEPER_URL"), os.Getenv("KEEPER_TOKEN"); u != "" && t != "" {
		return &keeperAPI{url: strings.TrimRight(u, "/"), token: t}, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "keeper", "config.json"))
	if err != nil {
		return nil, fmt.Errorf("read the keeper CLI's login (keeper login first): %w", err)
	}
	var cfg struct {
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil || cfg.URL == "" || cfg.Token == "" {
		return nil, fmt.Errorf("the keeper CLI's login is incomplete; run keeper login")
	}
	return &keeperAPI{url: strings.TrimRight(cfg.URL, "/"), token: cfg.Token}, nil
}

func (k *keeperAPI) do(method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		r = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, k.url+"/keeper-api"+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+k.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s %s", method, path, resp.Status, strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func getJSON(url string, out any) error {
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s answered %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func gitCommit() string {
	out, err := sh.Output("git", "rev-parse", "--short", "HEAD")
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(out)
}

// embedFrontend replaces backend/webui/dist with frontend/dist.
func embedFrontend() error {
	if _, err := os.Stat(filepath.Join("frontend", "dist", "index.html")); err != nil {
		return fmt.Errorf("frontend/dist/index.html missing; run mage buildFrontend: %w", err)
	}
	entries, err := os.ReadDir(embedDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Name() != ".gitkeep" {
			if err := os.RemoveAll(filepath.Join(embedDir, e.Name())); err != nil {
				return err
			}
		}
	}
	src := filepath.Join("frontend", "dist")
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(embedDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return sh.Copy(target, path)
	})
}
