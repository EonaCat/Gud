package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
	"regexp"
)

const (
	GUD_DIR           = ".gud"
	BRANCHES_DIR      = ".gud/branches"
	CURRENT_BRANCH    = ".gud/HEAD"
	STAGING_FILE      = ".gud/staging_area"
	CURRENT_BRANCH_FILE = ".gud/HEAD"
	REMOTE_DIR        = ".gud_remote"
	TAGS_FILE         = ".gud/tags"
	LOG_FILE          = ".gud/logs"
	COMMITS_DIR       = ".gud/commits"
	REMOTE_URL_FILE   = ".gud/remote_url"
	IGNORE_FILE       = ".gudignore"
	CONFIG_FILE       = ".gud/config.json"
)

type Commit struct {
	ID        string            `json:"id"`
	Message   string            `json:"message"`
	Timestamp string            `json:"timestamp"`
	Files     map[string]string `json:"files"`  // filepath -> content
	Branch    string            `json:"branch"`
}

type Config struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func showRemoteURL() {
	data, err := os.ReadFile(REMOTE_URL_FILE)
	if err != nil {
		fmt.Println("No remote URL configured.")
		return
	}
	fmt.Println("Remote URL:", string(data))
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: gud <command> [args]")
		return
	}
	cmd := os.Args[1]
	switch cmd {
	case "init":
		initRepo()
	case "add":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gud add <file>")
			return
		}
		addFileToStaging(os.Args[2])
	case "add-p":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gud add-p <file>")
			return
		}
		interactiveAdd(os.Args[2])
	case "unstage":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gud unstage <file>")
			return
		}
		unstageFile(os.Args[2])
	case "status":
		status()
	case "diff":
		diff()
	case "commit":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gud commit <message>")
			return
		}
		createCommit(strings.Join(os.Args[2:], " "))
	case "amend":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gud amend <new message>")
			return
		}
		amendLastCommit(strings.Join(os.Args[2:], " "))
	case "restore":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gud restore <commit_id>")
			return
		}
		restoreCommit(os.Args[2])
	case "branch":
		handleBranchCommand(os.Args[2:])
	case "merge":
		if len(os.Args) != 4 {
			fmt.Println("Usage: gud merge <base> <target>")
			return
		}
		mergeBranches(os.Args[2], os.Args[3])
	case "rebase":
		if len(os.Args) != 4 {
			fmt.Println("Usage: gud rebase <base> <target>")
			return
		}
		rebaseOnto(os.Args[2], os.Args[3])
	case "push":
		pushRemote()
	case "pull":
		pullRemote()
	case "log":
		if len(os.Args) == 3 {
			showFileHistory(os.Args[2])
		} else {
			logHistory()
		}
	case "tag":
		handleTagCommand(os.Args[2:])
	case "get-tag":
		if len(os.Args) != 3 {
			fmt.Println("Usage: gud get-tag <name>")
			return
		}
		getCommitByTag(os.Args[2])
	case "remote-url":
		if len(os.Args) == 3 {
			setRemoteURL(os.Args[2])
		} else {
			showRemoteURL()
		}
	case "clone":
		if len(os.Args) != 4 {
			fmt.Println("Usage: gud clone <remote_path> <target_dir>")
			return
		}
		cloneRepository(os.Args[2], os.Args[3])
	case "revert":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gud revert <commit-id>")
			return
		}
		revertTo(os.Args[2])
	case "config":
		if len(os.Args) < 4 {
			fmt.Println("Usage: gud config <username> <email>")
			return
		}
		saveUserConfig(os.Args[2], os.Args[3])
	default:
		fmt.Println("Unknown command:", cmd)
	}
}

func readIgnorePatterns() map[string]bool {
	patterns := make(map[string]bool)
	file, err := os.Open(IGNORE_FILE)
	if err != nil {
		return patterns
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		patterns[scanner.Text()] = true
	}
	return patterns
}

/* ----------------------------------------
   FEATURE 1: Undo Last Commit (Amend)
-------------------------------------------*/
func amendLastCommit(newMsg string) {
	last := latestCommit(currentBranch())
	if last == nil {
		fmt.Println("No commits to amend.")
		return
	}

	// Load staged files (if any) to update commit snapshot
	staged := loadStaging()
	if len(staged) > 0 {
		last.Files = staged
		os.Remove(STAGING_FILE)
	}

	last.Message = newMsg
	last.Timestamp = time.Now().Format(time.RFC3339)

	data, err := json.MarshalIndent(last, "", "  ")
	if err != nil {
		fmt.Println("Error encoding commit:", err)
		return
	}
	err = os.WriteFile(filepath.Join(COMMITS_DIR, last.ID+".json"), data, 0644)
	if err != nil {
		fmt.Println("Error writing commit file:", err)
		return
	}

	// Update log (append amend note)
	appendLog(fmt.Sprintf("%s [%s] (amended) %s\n", last.ID, last.Branch, newMsg))
	fmt.Println("Amended commit:", last.ID)
}

/* ----------------------------------------
   FEATURE 2: Show Commit History With Pretty Graph
-------------------------------------------*/
func logHistory() {
	data, err := os.ReadFile(LOG_FILE)
	if err != nil {
		fmt.Println("No commits yet.")
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		fmt.Println("No commits yet.")
		return
	}

	// Simple pretty print with branch and commit IDs
	fmt.Println("Commit history:")
	for i, line := range lines {
		// Format: ID [branch] message
		// Example: 1234567890 [main] Initial commit
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 3 {
			fmt.Println(line)
			continue
		}
		id := parts[0]
		branch := strings.Trim(parts[1], "[]")
		msg := parts[2]

		indent := strings.Repeat("| ", i)
		fmt.Printf("%s* %s (%s) %s\n", indent, id[:7], branch, msg)
	}
}

/* ----------------------------------------
   FEATURE 3: Tag commits (create/list/delete)
-------------------------------------------*/
func handleTagCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gud tag <create|list|delete> [args]")
		return
	}
	switch args[0] {
	case "create":
		if len(args) != 3 {
			fmt.Println("Usage: gud tag create <name> <commit_id>")
			return
		}
		tagCommit(args[1], args[2])
	case "list":
		listTags()
	case "delete":
		if len(args) != 2 {
			fmt.Println("Usage: gud tag delete <name>")
			return
		}
		deleteTag(args[1])
	default:
		fmt.Println("Unknown tag command:", args[0])
	}
}

func tagCommit(tag, commitID string) {
	tags := loadTags()
	tags[tag] = commitID
	saveTags(tags)
	fmt.Printf("Tagged commit %s as '%s'\n", commitID, tag)
}

func listTags() {
	tags := loadTags()
	if len(tags) == 0 {
		fmt.Println("No tags found.")
		return
	}
	fmt.Println("Tags:")
	for tag, commit := range tags {
		fmt.Printf(" - %s: %s\n", tag, commit)
	}
}

func deleteTag(tag string) {
	tags := loadTags()
	if _, ok := tags[tag]; !ok {
		fmt.Println("Tag not found:", tag)
		return
	}
	delete(tags, tag)
	saveTags(tags)
	fmt.Println("Deleted tag:", tag)
}

func loadTags() map[string]string {
	tags := make(map[string]string)
	data, err := os.ReadFile(TAGS_FILE)
	if err == nil {
		json.Unmarshal(data, &tags)
	}
	return tags
}

func saveTags(tags map[string]string) {
	data, _ := json.MarshalIndent(tags, "", "  ")
	os.WriteFile(TAGS_FILE, data, 0644)
}

/* ----------------------------------------
   FEATURE 4: Undo Add (Unstage file)
-------------------------------------------*/
func unstageFile(file string) {
	staged := loadStaging()
	if _, ok := staged[file]; !ok {
		fmt.Println("File is not staged:", file)
		return
	}
	delete(staged, file)
	saveStaging(staged)
	fmt.Println("Unstaged:", file)
}

/* ----------------------------------------
   FEATURE 5: Interactive Staging (add -p)
-------------------------------------------*/
func interactiveAdd(file string) {
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Println("File not found:", file)
		return
	}

	lines := strings.Split(string(content), "\n")
	staged := loadStaging()

	reader := bufio.NewReader(os.Stdin)
	var selectedLines []string

	fmt.Println("Interactive add for", file)
	for i, line := range lines {
		fmt.Printf("%5d: %s\n", i+1, line)
		fmt.Print("Stage this line? (y/n/q) ")
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(resp)
		if resp == "q" {
			break
		}
		if resp == "y" {
			selectedLines = append(selectedLines, line)
		} else {
			selectedLines = append(selectedLines, "") // blank line unstaged
		}
	}

	// Join only staged lines, ignoring blanks for unstaged lines
	filteredLines := []string{}
	for _, l := range selectedLines {
		if l != "" {
			filteredLines = append(filteredLines, l)
		}
	}
	if len(filteredLines) == 0 {
		fmt.Println("No lines staged.")
		return
	}
	staged[file] = strings.Join(filteredLines, "\n")
	saveStaging(staged)
	fmt.Println("Interactive add done for", file)
}

/* ----------------------------------------
   FEATURE 6: Show file history (file-specific commit log)
-------------------------------------------*/
func showFileHistory(filename string) {
	entries, err := ioutil.ReadDir(COMMITS_DIR)
	if err != nil {
		fmt.Println("Error reading commits:", err)
		return
	}

	type fileCommit struct {
		CommitID  string
		Timestamp string
		Message   string
	}

	var history []fileCommit

	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(COMMITS_DIR, entry.Name()))
		if err != nil {
			continue
		}
		var c Commit
		json.Unmarshal(data, &c)
		if _, ok := c.Files[filename]; ok {
			history = append(history, fileCommit{c.ID, c.Timestamp, c.Message})
		}
	}

	if len(history) == 0 {
		fmt.Println("No history for file:", filename)
		return
	}

	// Print history
	fmt.Printf("History for file: %s\n", filename)
	for i := len(history) - 1; i >= 0; i-- {
		c := history[i]
		fmt.Printf("- %s (%s): %s\n", c.CommitID[:7], c.Timestamp, c.Message)
	}
}

/* ----------------------------------------
   FEATURE 7: Checkout specific file from commit/tag
-------------------------------------------*/
func checkoutFile(commitOrTag, file string) {
	commitID := commitOrTag
	tags := loadTags()
	if cid, ok := tags[commitOrTag]; ok {
		commitID = cid
	}

	data, err := os.ReadFile(filepath.Join(COMMITS_DIR, commitID+".json"))
	if err != nil {
		fmt.Println("Commit not found:", commitID)
		return
	}

	var c Commit
	json.Unmarshal(data, &c)
	content, ok := c.Files[file]
	if !ok {
		fmt.Println("File not found in commit:", file)
		return
	}

	err = os.WriteFile(file, []byte(content), 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
		return
	}
	fmt.Printf("Checked out %s from %s\n", file, commitOrTag)
}

/* ----------------------------------------
   FEATURE 8: List remote commits before pull (preview remote changes)
-------------------------------------------*/
func previewRemoteCommits() {
	localLatest := latestCommit(currentBranch())
	remoteCommits := loadRemoteCommits()
	fmt.Println("Remote commits not in local:")

	for _, c := range remoteCommits {
		if localLatest == nil || c.Timestamp > localLatest.Timestamp {
			fmt.Printf("- %s: %s\n", c.ID[:7], c.Message)
		}
	}
}

/* ----------------------------------------
   FEATURE 9: Branch deletion
-------------------------------------------*/
func handleBranchCommand(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: gud branch <create|list|delete> [args]")
		return
	}
	switch args[0] {
	case "create":
		if len(args) != 2 {
			fmt.Println("Usage: gud branch create <name>")
			return
		}
		createBranch(args[1])
	case "list":
		listBranches()
	case "delete":
		if len(args) != 2 {
			fmt.Println("Usage: gud branch delete <name>")
			return
		}
		deleteBranch(args[1])
	default:
		fmt.Println("Unknown branch command:", args[0])
	}
}

func createBranch(name string) {
	branches := loadBranches()
	if _, ok := branches[name]; ok {
		fmt.Println("Branch already exists:", name)
		return
	}
	branches[name] = currentBranchHead()
	saveBranches(branches)
	fmt.Println("Created branch:", name)
}

func listBranches() {
	branches := loadBranches()
	current := currentBranch()
	for b := range branches {
		marker := " "
		if b == current {
			marker = "*"
		}
		fmt.Printf("%s %s\n", marker, b)
	}
}

func deleteBranch(name string) {
	branches := loadBranches()
	if _, ok := branches[name]; !ok {
		fmt.Println("Branch not found:", name)
		return
	}
	if name == currentBranch() {
		fmt.Println("Cannot delete current branch.")
		return
	}
	delete(branches, name)
	saveBranches(branches)
	fmt.Println("Deleted branch:", name)
}

/* ----------------------------------------
   FEATURE 10: Better ignore patterns (support glob)
-------------------------------------------*/
func loadIgnorePatterns() []string {
	data, err := os.ReadFile(IGNORE_FILE)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var patterns []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			patterns = append(patterns, l)
		}
	}
	return patterns
}

func isIgnored(file string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, _ := filepath.Match(pattern, file)
		if matched {
			return true
		}
		// support ** pattern (recursive)
		if strings.Contains(pattern, "**") {
			reg := globToRegex(pattern)
			re := regexp.MustCompile(reg)
			if re.MatchString(file) {
				return true
			}
		}
	}
	return false
}

// Converts glob pattern with ** to regex
func globToRegex(pattern string) string {
	regex := regexp.QuoteMeta(pattern)
	regex = strings.ReplaceAll(regex, `\*\*`, `.*`)
	regex = "^" + regex + "$"
	return regex
}

/* ----------------------------------------
 Helper functions below (load/save commits, branches, staging, etc)
-------------------------------------------*/

func initRepo() {
	os.Mkdir(GUD_DIR, 0755)
	os.Mkdir(BRANCHES_DIR, 0755)
	os.Mkdir(COMMITS_DIR, 0755)
	os.WriteFile(CURRENT_BRANCH, []byte("main"), 0644)
	os.WriteFile(STAGING_FILE, []byte("{}"), 0644)
	os.WriteFile(TAGS_FILE, []byte("{}"), 0644)
	os.WriteFile(LOG_FILE, []byte(""), 0644)
	fmt.Println("Initialized empty gud repository")
}

func addFileToStaging(file string) {
	patterns := loadIgnorePatterns()
	if isIgnored(file, patterns) {
		fmt.Println("File ignored:", file)
		return
	}
	content, err := os.ReadFile(file)
	if err != nil {
		fmt.Println("File not found:", file)
		return
	}
	staged := loadStaging()
	staged[file] = string(content)
	saveStaging(staged)
	fmt.Println("Added to staging:", file)
}

func loadStaging() map[string]string {
	data, err := os.ReadFile(STAGING_FILE)
	if err != nil {
		return make(map[string]string)
	}
	var staging map[string]string
	json.Unmarshal(data, &staging)
	if staging == nil {
		staging = make(map[string]string)
	}
	return staging
}

func saveStaging(staging map[string]string) {
	data, _ := json.MarshalIndent(staging, "", "  ")
	os.WriteFile(STAGING_FILE, data, 0644)
}

func createCommit(msg string) {
	staged := loadStaging()
	if len(staged) == 0 {
		fmt.Println("Nothing to commit.")
		return
	}

	branch := currentBranch()
	last := latestCommit(branch)

	files := make(map[string]string)
	if last != nil {
		for k, v := range last.Files {
			files[k] = v
		}
	}

	for k, v := range staged {
		files[k] = v
	}

	id := fmt.Sprintf("%x", time.Now().UnixNano())
	c := Commit{
		ID:        id,
		Message:   msg,
		Timestamp: time.Now().Format(time.RFC3339),
		Files:     files,
		Branch:    branch,
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	os.WriteFile(filepath.Join(COMMITS_DIR, id+".json"), data, 0644)

	// update branch head
	branches := loadBranches()
	branches[branch] = id
	saveBranches(branches)

	// clear staging
	os.Remove(STAGING_FILE)

	appendLog(fmt.Sprintf("%s [%s] %s\n", id, branch, msg))
	fmt.Println("Committed:", id)
}

func latestCommit(branch string) *Commit {
	branches := loadBranches()
	head, ok := branches[branch]
	if !ok {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(COMMITS_DIR, head+".json"))
	if err != nil {
		return nil
	}
	var c Commit
	json.Unmarshal(data, &c)
	return &c
}

func loadBranches() map[string]string {
	data, err := os.ReadFile(BRANCHES_DIR + "/branches.json")
	if err != nil {
		return make(map[string]string)
	}
	var branches map[string]string
	json.Unmarshal(data, &branches)
	if branches == nil {
		branches = make(map[string]string)
	}
	return branches
}

func saveBranches(branches map[string]string) {
	os.MkdirAll(BRANCHES_DIR, 0755)
	data, _ := json.MarshalIndent(branches, "", "  ")
	os.WriteFile(BRANCHES_DIR+"/branches.json", data, 0644)
}

func currentBranch() string {
	data, err := os.ReadFile(CURRENT_BRANCH)
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(data))
}

func currentBranchHead() string {
	branches := loadBranches()
	return branches[currentBranch()]
}

func appendLog(line string) {
	f, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line)
}

func restoreCommit(commitID string) {
	data, err := os.ReadFile(filepath.Join(COMMITS_DIR, commitID+".json"))
	if err != nil {
		fmt.Println("Commit not found:", commitID)
		return
	}
	var c Commit
	json.Unmarshal(data, &c)

	for file, content := range c.Files {
		os.WriteFile(file, []byte(content), 0644)
	}
	fmt.Println("Restored commit:", commitID)
}

func loadRemoteCommits() []Commit {
	var commits []Commit
	files, err := ioutil.ReadDir(REMOTE_DIR + "/commits")
	if err != nil {
		return commits
	}
	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(REMOTE_DIR, "commits", f.Name()))
		if err != nil {
			continue
		}
		var c Commit
		json.Unmarshal(data, &c)
		commits = append(commits, c)
	}
	return commits
}

/* ----------------------------------------
 User config
-------------------------------------------*/
func saveUserConfig(username, email string) {
	cfg := Config{username, email}
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(CONFIG_FILE, data, 0644)
	fmt.Println("User config saved.")
}

func loadUserConfig() *Config {
	data, err := os.ReadFile(CONFIG_FILE)
	if err != nil {
		return nil
	}
	var cfg Config
	json.Unmarshal(data, &cfg)
	return &cfg
}

func pushRemote() {
	remoteCommitsDir := filepath.Join(REMOTE_DIR, "commits")
	err := os.MkdirAll(remoteCommitsDir, 0755)
	if err != nil {
		fmt.Println("Failed to create remote commits directory:", err)
		return
	}

	entries, err := ioutil.ReadDir(COMMITS_DIR)
	if err != nil {
		fmt.Println("Error reading commits directory:", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(COMMITS_DIR, entry.Name())
		dst := filepath.Join(remoteCommitsDir, entry.Name())

		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Println("Error reading commit file:", src, err)
			continue
		}

		err = os.WriteFile(dst, data, 0644)
		if err != nil {
			fmt.Println("Error writing to remote commit file:", dst, err)
		}
	}
	fmt.Println("Pushed commits to remote.")
}


func pullRemote() {
	remoteCommitsDir := filepath.Join(REMOTE_DIR, "commits")

	entries, err := ioutil.ReadDir(remoteCommitsDir)
	if err != nil {
		fmt.Println("Error reading remote commits directory:", err)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(remoteCommitsDir, entry.Name())
		dst := filepath.Join(COMMITS_DIR, entry.Name())

		if _, err := os.Stat(dst); err == nil {
			fmt.Printf("Commit %s already exists locally, skipping.\n", entry.Name())
			continue
		}

		data, err := os.ReadFile(src)
		if err != nil {
			fmt.Println("Error reading remote commit file:", src, err)
			continue
		}

		err = os.WriteFile(dst, data, 0644)
		if err != nil {
			fmt.Println("Error writing to local commit file:", dst, err)
		}
	}
	fmt.Println("Pulled commits from remote.")
}

func switchBranch(branch string) {
	os.WriteFile(CURRENT_BRANCH_FILE, []byte(branch), 0644)
	fmt.Println("Switched to branch:", branch)
}

func mergeBranches(base, target string) {
	fmt.Printf("Merging branch '%s' into '%s'\n", target, base)

	latestTarget := latestCommit(target)
	if latestTarget == nil {
		fmt.Println("No commits found on target branch:", target)
		return
	}

	// Restore target commit files
	for path, content := range latestTarget.Files {
		err := os.WriteFile(path, []byte(content), 0644)
		if err != nil {
			fmt.Println("Error writing file during merge:", path, err)
			return
		}
	}

	// Switch to base branch and commit the merge
	switchBranch(base)

	message := fmt.Sprintf("Merge branch '%s' into '%s'", target, base)
	createCommit(message)

	fmt.Println("Merge completed.")
}


func rebaseOnto(base, target string) {
	fmt.Printf("Rebasing branch '%s' onto '%s'\n", target, base)

	// For simplicity, reuse merge logic to simulate rebase
	mergeBranches(base, target)

	// Switch back to target branch
	switchBranch(target)

	fmt.Println("Rebase completed.")
}


func cloneRepository(remotePath, targetDir string) {
	err := os.MkdirAll(targetDir, 0755)
	if err != nil {
		fmt.Println("Failed to create target directory:", err)
		return
	}

	copyDir := func(src, dst string) error {
		return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			relPath, _ := filepath.Rel(src, path)
			targetPath := filepath.Join(dst, relPath)

			if info.IsDir() {
				return os.MkdirAll(targetPath, info.Mode())
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(targetPath, data, info.Mode())
		})
	}

	remoteGudDir := filepath.Join(remotePath, ".gud")
	targetGudDir := filepath.Join(targetDir, ".gud")

	err = copyDir(remoteGudDir, targetGudDir)
	if err != nil {
		fmt.Println("Error copying repository:", err)
		return
	}

	fmt.Println("Repository cloned to", targetDir)
}


func revertTo(commitID string) {
	commitPath := filepath.Join(COMMITS_DIR, commitID+".json")
	data, err := os.ReadFile(commitPath)
	if err != nil {
		fmt.Println("Commit not found:", commitID)
		return
	}

	var c Commit
	err = json.Unmarshal(data, &c)
	if err != nil {
		fmt.Println("Error parsing commit:", err)
		return
	}

	for file, content := range c.Files {
		err := os.WriteFile(file, []byte(content), 0644)
		if err != nil {
			fmt.Println("Error restoring file:", file, err)
			return
		}
	}

	fmt.Println("Reverted working directory to commit:", commitID)
}

func getCommitByTag(tag string) {
	tags := loadTags()
	id, ok := tags[tag]
	if !ok {
		fmt.Println("Tag not found")
		return
	}
	restoreCommit(id)
}


func diff() {
	current := getWorkingFiles()
	last := getLastCommitFiles()

	fmt.Println("Differences:")
	for file, currentContent := range current {
		lastContent, exists := last[file]
		if !exists {
			fmt.Println("+", file) // New file
		} else if currentContent != lastContent {
			fmt.Println("~", file) // Modified file
		}
	}

	for file := range last {
		if _, exists := current[file]; !exists {
			fmt.Println("-", file) // Deleted file
		}
	}
}

func getLastCommitFiles() map[string]string {
	entries, _ := ioutil.ReadDir(COMMITS_DIR)
	var last Commit
	for _, entry := range entries {
		data, _ := os.ReadFile(filepath.Join(COMMITS_DIR, entry.Name()))
		var c Commit
		json.Unmarshal(data, &c)
		if last.Timestamp < c.Timestamp {
			last = c
		}
	}
	return last.Files
}

func getWorkingFiles() map[string]string {
	files := make(map[string]string)
	ignores := readIgnorePatterns()
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
	
	var ignoresList []string
	for key := range ignores {
	    ignoresList = append(ignoresList, key)
	}
	// then call isIgnored with ignoresList
	if strings.HasPrefix(path, ".trackly") || info.IsDir() || isIgnored(path, ignoresList) {
	    return nil
	}
		content, _ := os.ReadFile(path)
		files[path] = string(content)
		return nil
	})
	return files
}

func getStagedFiles() map[string]string {
	staged := make(map[string]string)
	data, err := os.ReadFile(STAGING_FILE)
	if err != nil {
		return staged
	}
	json.Unmarshal(data, &staged)
	return staged
}

func setRemoteURL(url string) {
	os.WriteFile(REMOTE_URL_FILE, []byte(url), 0644)
	fmt.Println("Remote URL set to:", url)
}

func status() {
	current := getWorkingFiles()
	staged := getStagedFiles()
	last := getLastCommitFiles()

	fmt.Println("Modified files:")
	for file, content := range current {
		if lastContent, ok := last[file]; ok && content != lastContent && staged[file] != content {
			fmt.Println(" *", file)
		}
	}

	fmt.Println("\nStaged files:")
	for file := range staged {
		fmt.Println(" +", file)
	}

	fmt.Println("\nUntracked files:")
	for file := range current {
		if _, inLast := last[file]; !inLast {
			if _, stagedAlready := staged[file]; !stagedAlready {
				fmt.Println(" ?", file)
			}
		}
	}
}

