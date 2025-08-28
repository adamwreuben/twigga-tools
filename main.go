package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/adamwreuben/twiggatools/models"
	"github.com/adamwreuben/twiggatools/utils"
	"github.com/spf13/cobra"
)

var Cfg *models.Config
var CfgFile string

var APIClient *utils.APIClient

var rootCmd = &cobra.Command{
	Use:   "twigga",
	Short: "Twigga CLI — manage twigga projects (auth, storage, hosting)",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Twigga to authenticate",
	RunE: func(cmd *cobra.Command, args []string) error {

		port := 53682
		redirect := fmt.Sprintf("http://localhost:%d/callback", port)

		if !Cfg.Status {

			Cfg, err := utils.EnsureConfig(CfgFile)
			if err != nil {
				fmt.Println("err from EnsureConfig: ", err.Error())
			}

			authURL, err := APIClient.Authenticate(context.Background(), redirect)
			if err != nil {
				return err
			}
			fmt.Println("Open the following URL in your browser ):")
			fmt.Println(authURL)

			// open browser
			_ = openBrowser(authURL)

			// start local server to capture callback
			tokenCh := make(chan string, 1)
			go startCallbackServer(port, tokenCh)

			// wait for token with a timeout
			select {
			case tok := <-tokenCh:
				if tok == "" {
					return fmt.Errorf("no token captured — check your browser redirect or copy token manually")
				}

				if err := utils.SaveToken(CfgFile, Cfg, tok); err != nil {
					return err
				}

			case <-time.After(120 * time.Second):
				return fmt.Errorf("timeout! — try again 'twigga login' again or open the auth url manually")
			}
		} else {
			fmt.Println("Already loggedin — type 'twigga logout' and try again 'twigga login'.")
			return nil
		}

		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out twigga user",
	RunE: func(cmd *cobra.Command, args []string) error {

		Cfg.Token = ""
		Cfg.Status = false
		Cfg.ProjectId = ""

		err := utils.SaveConfig(CfgFile, Cfg)
		if err != nil {
			return err
		}
		fmt.Println("Logged out successful.")
		return nil
	},
}

var projectListCommand = &cobra.Command{
	Use:   "projects",
	Short: "List all projects:",
	RunE: func(cmd *cobra.Command, args []string) error {

		if Cfg.Status {
			userData, err := APIClient.GetTokenData(context.Background(), Cfg.Token)
			if err != nil {
				fmt.Println("Err: ", err.Error())
				return err
			}

			userId := userData["id"].(string)

			dataToFilter := map[string]interface{}{
				"members": []string{userId},
			}

			resp, err := APIClient.QueryDocuments(context.Background(), "Twigga", "Projects", dataToFilter)

			if err != nil {
				fmt.Println("err: ", err)
				return err
			}

			documents := resp["documents"].([]interface{})
			fmt.Println("List of projects(" + strconv.Itoa(len(documents)) + ")")
			fmt.Println("------------------------------------------------------")
			fmt.Println("ProjectName                 ProjectId")
			for _, doc := range documents {
				docMap := doc.(map[string]interface{})
				name := docMap["projectName"].(string)
				projId := docMap["projectId"].(string)

				fmt.Println(name, "               ", projId)
			}

			return nil
		} else {
			fmt.Println("You need to be authenticated, try 'twigga login'")
		}

		return nil
	},
}

var projectUseCmd = &cobra.Command{
	Use:   "use",
	Short: "Set projectId",
	RunE: func(cmd *cobra.Command, args []string) error {

		if Cfg.Status {
			projectId := args[0]

			hostingBucket := "hosting-" + strings.ToLower(projectId)
			Cfg.ProjectId = projectId

			_, err := APIClient.AddBucket(context.Background(), hostingBucket)
			if err != nil {
				log.Println("ERROR ProjectUse: ", err.Error())
			}

			//make it public accessible
			err = APIClient.SetBucketPolicy(context.Background(), hostingBucket, "public")
			if err != nil {
				log.Println("ERROR SetBucketPolicy: ", err.Error())
			}

			err = utils.SaveConfig(CfgFile, Cfg)
			if err != nil {
				return err
			}

			fmt.Println("Project is set")

		} else {
			fmt.Println("You need to be authenticated, try 'twigga login'")
		}

		return nil
	},
}

var activeprojectCmd = &cobra.Command{
	Use:   "project",
	Short: "List active project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if Cfg.Status {

			if Cfg.ProjectId != "" {
				fmt.Println("Active project with ID: ", Cfg.ProjectId)
				return nil
			} else {
				fmt.Println("no active project, Try 'twigga use [projectId]' or list projects 'twigga projects'")
				return nil
			}

		} else {
			fmt.Println("you need to be authenticated, try 'twigga login'")
			return nil
		}
	},
}

var bucketCreateCmd = &cobra.Command{
	Use:   "bucket create [name]",
	Short: "Create a bucket",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {

		folderName := args[1]

		bucketDataToFilter := map[string]interface{}{
			"projectId": Cfg.ProjectId,
			"folder":    folderName,
		}

		queryResp, _ := APIClient.QueryDocuments(context.Background(), "Twigga", "Buckets", bucketDataToFilter)

		if queryResp["documents"] == nil {

			folderId := utils.GenerateDocumentID()

			dataToPush := map[string]interface{}{
				"folder":    folderName,
				"folderId":  folderId,
				"projectId": Cfg.ProjectId,
				"createdAt": time.Now(),
			}

			APIClient.CreateDocumentAuto(context.Background(), "Twigga", "Buckets", dataToPush)
			_, err := APIClient.AddBucket(context.Background(), folderName)
			if err != nil {
				fmt.Println("Message:", err.Error())
				return nil
			}

			fmt.Println("Message: bucket created")
			return nil
		} else {
			fmt.Println("Message: folder ", folderName, " exists already!")
			return nil
		}

	},
}

var bucketListCmd = &cobra.Command{
	Use:   "buckets",
	Args:  cobra.NoArgs,
	Short: "List buckets",
	RunE: func(cmd *cobra.Command, args []string) error {

		dataToFilter := map[string]interface{}{
			"projectId": Cfg.ProjectId,
		}

		filterRes, _ := APIClient.QueryDocuments(context.Background(), "Twigga", "Buckets", dataToFilter)

		if filterRes["documents"] != nil {
			fmt.Println("LIST OF BUCKETS")
			fmt.Println("FolderName       FolderId")
			fmt.Println("---------------------------------")
			for _, doc := range filterRes["documents"].([]interface{}) {
				docData := doc.(map[string]interface{})
				folderName := docData["folder"]
				folderId := docData["folderId"]
				fmt.Println(folderName, " ", folderId)
			}
		}

		return nil
	},
}

var uploadCmd = &cobra.Command{
	Use:   "storage upload [fbucketile-or-dir]",
	Args:  cobra.ExactArgs(3),
	Short: "Upload file or directory to bucket (preserves relative paths)",
	RunE: func(cmd *cobra.Command, args []string) error {
		bucket := args[1]
		path := args[2]

		info, err := os.Stat(path)
		if err != nil {
			return err
		}

		var files []string
		baseDir := path
		if info.IsDir() {
			baseDir = path
			err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					files = append(files, p)
				}
				return nil
			})
			if err != nil {
				return err
			}
		} else {
			files = []string{path}
			baseDir = filepath.Dir(path)
		}

		fmt.Printf("Uploading %d files to bucket %s ...\n", len(files), bucket)
		uploaded, err := APIClient.UploadFiles(context.Background(), bucket, files, baseDir)
		if err != nil {
			return err
		}
		fmt.Println("Uploaded:")
		for _, u := range uploaded {
			fmt.Println(" -", u)
		}
		return nil
	},
}

var hostingDeployCmd = &cobra.Command{
	Use:   "deploy <dir>",
	Args:  cobra.ExactArgs(1),
	Short: "Deploy static site directory to hosting bucket",
	RunE: func(cmd *cobra.Command, args []string) error {
		if Cfg.ProjectId == "" {
			fmt.Println("No active project. Try 'twigga projects'")
			return nil
		}

		site := Cfg.ProjectId
		dir := args[0]

		// Validate directory exists
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("invalid directory: %s", dir)
		}

		bucket := "hosting-" + strings.ToLower(site)

		// Compute release hash (version)
		files := []string{}
		err = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return err
		}

		version, err := utils.ComputeReleaseHash(dir, files)
		if err != nil {
			return fmt.Errorf("compute hash: %v", err)
		}
		version = "v-" + version
		log.Printf("Release version: %s\n", version)

		// Upload to hosting API
		fmt.Printf("Deploying %d files to %s ...\n", len(files), bucket)
		uploaded, err := APIClient.UploadSiteVersion(context.Background(), bucket, site, version, dir)
		if err != nil {
			return fmt.Errorf("upload failed: %v", err)
		}
		for _, f := range uploaded {
			fmt.Println(" -", f)
		}

		// Point "main" channel to this version
		err = APIClient.PointChannel(context.Background(), bucket, site, "main", version)
		if err != nil {
			return fmt.Errorf("failed to point main channel: %v", err)
		}

		fmt.Printf("Site deployed and pointed: https://%s.apps.bongocloud.co.tz\n", site)
		return nil
	},
}

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("Cannot find home dir:", err)
		os.Exit(1)
	}

	Cfg = &models.Config{
		BaseURL:        "https://twiga.bongocloud.co.tz",
		AccountBaseURL: "https://account.bongocloud.co.tz",
		Status:         false,
		ProjectId:      "",
		Token:          "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJkYXRhIjp7ImFwcElkIjoidHdpZ2dhdG9vbHMiLCJhcHBTZWNyZXQiOiI1MjAxYzhmNC1hYjE3LTRmZTQtOTcxZC1lZGMwMzgzOTMwZGMiLCJleHAiOjE3NTg1Mzg1NTR9LCJleHAiOjE3NTg1Mzg1NTR9.wSJM1YnC4VdOGzSUmZ3r8v0uOJGA7g9L2X3fgQkdt6ciafX9SLnVK8zkExjC5arrutD4tRolyeUg-YpTJaJS4mOdxL3LMX8uulnbGUhpEbrawFMyGuStsZ7dgLxFpUxlAHbaQfutRFnoPYZnsjqmhWsgeW44taDe0S7HaypNqJJsNXK21iA-8-bToFKepTbLeKl9jCLfseyyGfrFcuQBXjuhjnJiwfQXFkKeoZ8-aE86fdwidCpbOmEEf9z-XwDwo_QzzbTyQh7Npr0MQOggXlVWF7TRhDqQa4X0EH4_ErmIGZEC9W57gvKiShdZYrhl2VYtgwHP1bd7UeWr6cw-Pw",
	}

	CfgFile = filepath.Join(home, ".twigga", "config.json")

	fileConf, err := utils.LoadConfig(CfgFile)
	if err != nil {
		fmt.Println("Error while loading configs: ", err.Error())
	}

	if fileConf.Status {
		Cfg = fileConf
	}

	APIClient = utils.NewAPIClientFromConfig(Cfg)

	setSubCommand()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println("Execute: ", err)
		os.Exit(1)
	}

}

func setSubCommand() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(projectListCommand)
	rootCmd.AddCommand(activeprojectCmd)
	rootCmd.AddCommand(projectUseCmd)
	rootCmd.AddCommand(bucketCreateCmd)
	rootCmd.AddCommand(bucketListCmd)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(hostingDeployCmd)
}

// helper to open a browser
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default: // linux etc
		cmd = "xdg-open"
		args = []string{url}
	}
	return execCommand(cmd, args...)
}

func execCommand(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// startCallbackServer listens for /callback?token=... and sends token into tokenCh
func startCallbackServer(port int, tokenCh chan string) {
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		token := q.Get("token")
		if token == "" {
			for k := range q {
				if strings.Contains(strings.ToLower(k), "token") {
					token = q.Get(k)
					break
				}
			}
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		if token == "" {
			fmt.Fprint(w, errorHTML())
			tokenCh <- ""
		} else {
			fmt.Fprint(w, successHTML())
			tokenCh <- token
		}

		// shutdown server gracefully
		go server.Shutdown(context.Background())
	})

	_ = server.ListenAndServe()
}

func successHTML() string {
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Login successful • Twigga</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
  :root { --bg:#ffffff; --card:#ffffff; --text:#0f172a; --muted:#64748b; --border:#e2e8f0; --accent:#16a34a; --btn:#F3BD50; --btn-text:#fff; }
  * { box-sizing: border-box; }
  html, body { height: 100%; }
  body {
    margin: 0; background: var(--bg); color: var(--text); font: 14px/1.5 system-ui, -apple-system, Segoe UI, Roboto, Ubuntu, Cantarell, Noto Sans, Arial, "Apple Color Emoji", "Segoe UI Emoji";
    display: grid; place-items: center; padding: 24px;
  }
  .card {
    width: 100%; max-width: 440px; background: var(--card); border: 1px solid var(--border);
    border-radius: 16px; padding: 28px; box-shadow: 0 10px 30px rgba(2,6,23,.06);
    text-align: center;
  }
  
  h1 { margin: 8px 0 4px; font-size: 20px; }
  p  { margin: 8px 0 0; color: var(--muted); }
  .ok {
    width: 64px; height: 64px; border-radius: 50%; background: rgba(22,163,74,.1);
    display: inline-grid; place-items: center; color: var(--accent); margin: 4px auto 8px;
  }
  .ok svg { width: 28px; height: 28px; }
  .btn {
    display: inline-block; padding: 10px 14px; border-radius: 12px; border: 1px solid transparent;
    background: var(--btn); color: var(--btn-text); text-decoration: none; font-weight: 600; margin-top: 16px;
    cursor: pointer;
  }
  .hint { margin-top: 10px; font-size: 12px; color: var(--muted); }
</style>
</head>
<body>
  <div class="card" role="status" aria-live="polite">
    
    <h1>You're logged in to Twigga</h1>
    <p>Authentication completed successfully. You can close this window.</p>
  </div>

</body>
</html>`
}

func errorHTML() string {
	return `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Login error • Twigga</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
  :root { --bg:#ffffff; --card:#ffffff; --text:#0f172a; --muted:#64748b; --border:#e2e8f0; --accent:#ef4444; --btn:#0ea5e9; --btn-text:#fff; }
  * { box-sizing: border-box; } html, body { height: 100%; }
  body { margin: 0; background: var(--bg); color: var(--text); font: 14px/1.5 system-ui, -apple-system, Segoe UI, Roboto, Ubuntu, Cantarell, Noto Sans, Arial; display: grid; place-items: center; padding: 24px; }
  .card { width: 100%; max-width: 440px; background: var(--card); border: 1px solid var(--border); border-radius: 16px; padding: 28px; box-shadow: 0 10px 30px rgba(2,6,23,.06); text-align: center; }
  .warn { width: 64px; height: 64px; border-radius: 50%; background: rgba(239,68,68,.1); display: inline-grid; place-items: center; color: var(--accent); margin: 4px auto 8px; }
  .warn svg { width: 28px; height: 28px; }
  h1 { margin: 8px 0 4px; font-size: 20px; }
  p  { margin: 8px 0 0; color: var(--muted); }
</style>
</head>
<body>
  <div class="card">
    
    <h1>We couldn't complete login</h1>
    <p>No token was found in the callback URL. Please try again or copy the full URL and run <code>twigga login</code> again.</p>
  </div>
</body>
</html>`
}
