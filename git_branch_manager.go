package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

const (
	AppName = "ggm"
)

var (
	title     func(string, ...interface{})
	info      func(string, ...interface{})
	warn      func(string, ...interface{})
	status    func(string, ...interface{})
	lastColor color.Attribute
)

func init() {
	cyan := color.New(color.FgCyan).PrintfFunc()
	hiCyan := color.New(color.FgHiCyan).PrintfFunc()
	t := color.New(color.FgGreen, color.Bold).PrintfFunc()
	title = func(format string, a ...interface{}) {
		t("\n"+format+"\n", a...)
	}

	s := color.New(color.FgBlue, color.Bold).PrintfFunc()
	status = func(format string, a ...interface{}) {
		s("\n"+format+"\n\n", a...)
	}

	info = func(format string, a ...interface{}) {
		if lastColor == color.FgCyan {
			hiCyan(format+"\n", a...)
			lastColor = color.FgHiCyan
		} else {
			cyan(format+"\n", a...)
			lastColor = color.FgCyan
		}
	}

	w := color.New(color.FgYellow, color.Bold).PrintfFunc()
	warn = func(format string, a ...interface{}) {
		w(format+"\n", a...)
	}
}

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		showHelp()
		os.Exit(0)
	}

	switch args[0] {
	case "--help", "-h":
		showHelp()
		return
	case "list":
		listSortedBranches()
	case "complete":
		if len(args) < 2 {
			log.Fatalf("The 'complete' command requires a shell type argument (bash or zsh)")
		}
		generateCompletionScript(args[1])
	case "generate-completion":
		if len(args) < 3 {
			log.Fatalf("Usage: %s generate-completion <bash|zsh> <output-file>", AppName)
		}
		generateCompletionFile(args[1], args[2])
	case "complete-branches":
		// Used by shell completion to get branch names
		fmt.Fprintf(os.Stderr, "==> complete-branches called\n")
		branches, _, err := listBranches()
		if err != nil {
			fmt.Fprintf(os.Stderr, "==> Error listing branches: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "==> Found %d branches\n", len(branches))
		for _, branch := range branches {
			fmt.Println(branch)
		}
	case "keep", "Keep":
		if len(args) < 2 {
			log.Fatalf("Usage: %s keep|Keep [branches to keep...]", AppName)
		}
		force := args[0] == "Keep"
		keepBranches(args[1:], force)
	case "delete", "Delete":
		if len(args) < 2 {
			log.Fatalf("Usage: %s delete|Delete [pattern or indexes]", AppName)
		}
		force := args[0] == "Delete"
		
		// Check if the argument is an index specification
		if isIndexSpec(args[1]) {
			deleteBranchesByIndexes(args[1], force)
		} else {
			deleteBranchesByPattern(args[1], force)
		}
		listSortedBranches()
	default:
		log.Fatalf("Invalid command. Use 'list', 'keep', 'Keep', 'delete', 'Delete', 'complete', 'generate-completion', '--help', or '-h'.")
	}
}

func confirmDeletion() bool {
	for {
		warn("\nType 'yes' to confirm deletion or 'no' to cancel:\n")
		var input string
		fmt.Scanln(&input)
		fmt.Println() // Print a newline
		if input == "yes" {
			return true
		} else if input == "no" {
			status("Deletion cancelled")
			return false
		}
	}
}

func _deleteBranches(branches []string, force bool) map[string]string {
	failed := make(map[string]string)
	branchCount := len(branches)
	if branchCount == 1 {
		title("Deleting branch %s...", branches[0])
	} else {
		title("Deleting %d branches...", branchCount)
	}
	for _, branch := range branches {
		err := deleteBranch(branch, force)
		if err != nil {
			failed[branch] = err.Error()
		}
	}
	return failed
}

func keepBranches(branchesToKeep []string, force bool) {
	allBranches, currentBranch, err := listBranches()
	if err != nil {
		warn("Error listing branches:", err)
		os.Exit(1)
	}

	var branchesToDelete []string
	for _, branch := range allBranches {
		if branch != "" && !contains(branchesToKeep, branch) {
			branchesToDelete = append(branchesToDelete, branch)
		}
	}

	confirmAndDeleteBranches(branchesToDelete, currentBranch, force)
}

func confirmAndDeleteBranches(branchesToDelete []string, currentBranch string, force bool) bool {
	// Filter out the current branch from the branches to delete
	filteredBranches := filterCurrentBranch(branchesToDelete, currentBranch)

	if len(filteredBranches) == 0 {
		status("No branches to delete.")
		return false
	}

	yes := confirmBranchesToDelete(filteredBranches)
	if !yes {
		return false
	}

	deleteBranches(filteredBranches, force)
	return true
}

func filterCurrentBranch(branchesToDelete []string, currentBranch string) []string {
	var filteredBranches []string
	currentBranchFiltered := false
	for _, branch := range branchesToDelete {
		if branch == currentBranch {
			currentBranchFiltered = true
		} else {
			filteredBranches = append(filteredBranches, branch)
		}
	}

	if currentBranchFiltered {
		status("Current branch (" + currentBranch + ") cannot be deleted.")
	}

	return filteredBranches
}

func deleteBranchesByPattern(pattern string, force bool) {
	branches, currentBranch, err := listBranches()
	if err != nil {
		log.Fatal("Error listing branches:", err)
	}

	isPrefixWildcard := strings.HasPrefix(pattern, "*")
	isSuffixWildcard := strings.HasSuffix(pattern, "*")
	pattern = strings.Trim(pattern, "*")

	var toDelete []string
	for _, branch := range branches {
		var match bool
		switch {
		case isPrefixWildcard && isSuffixWildcard:
			match = strings.Contains(branch, pattern)
		case isPrefixWildcard:
			match = strings.HasSuffix(branch, pattern)
		case isSuffixWildcard:
			match = strings.HasPrefix(branch, pattern)
		default:
			match = branch == pattern
		}

		if match {
			toDelete = append(toDelete, branch)
		}
	}

	if len(toDelete) == 0 {
		status("No branches match the given pattern.")
		return
	}

	confirmAndDeleteBranches(toDelete, currentBranch, force)
}

func deleteBranches(toDelete []string, force bool) {
	failed := _deleteBranches(toDelete, force)
	deletedCount := len(toDelete) - len(failed)

	if len(failed) > 0 {
		status("\n\nFailed to delete the following branches:")
		for branch, errMsg := range failed {
			warn("Branch: %s - Error: %s", branch, errMsg)
		}
	}

	deletedCountStr := "branches"
	toDeleteStr := "branches"

	if deletedCount == 1 {
		deletedCountStr = "branch"
	}

	if len(toDelete) == 1 {
		toDeleteStr = "branch"
	}

	status("\n%d out of %d %s deleted.\n", deletedCount, len(toDelete), toDeleteStr)
	failDeleteCount := len(toDelete) - deletedCount
	if failDeleteCount > 0 {
		warn("%d %s were not deleted due to errors.\n", failDeleteCount, deletedCountStr)
	}
}

func infoBranches(branches []string) {
	for i, branch := range branches {
		info("%2d. %s", i+1, branch)
	}
}

func confirmBranchesToDelete(toDelete []string) bool {
	if len(toDelete) == 1 {
		title("The following branch matches the pattern and will be deleted:")
	} else {
		title("The following branches match the pattern and will be deleted:")
	}

	infoBranches(toDelete)

	return confirmDeletion()
}

func listSortedBranches() {
	branches, _, err := listBranches()
	if err != nil {
		warn("Error listing branches: %s", err)
		os.Exit(1)
	}

	sort.Strings(branches)
	titleString := "Branches"
	if len(branches) == 1 {
		titleString = "Branch"
	}
	title(titleString)
	infoBranches(branches)
}

func listBranches() ([]string, string, error) {
	cmd := exec.Command("git", "branch")
	output, err := cmd.Output()
	if err != nil {
		return nil, "", err
	}

	branches := strings.Split(string(output), "\n")
	var currentBranch string
	var nonEmptyBranches []string

	for _, branch := range branches {
		branch = strings.TrimSpace(branch)
		if strings.HasPrefix(branch, "*") {
			branch = strings.TrimSpace(branch[1:])
			currentBranch = branch
		}
		if branch != "" {
			nonEmptyBranches = append(nonEmptyBranches, branch)
		}
	}

	return nonEmptyBranches, currentBranch, nil
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}

	_, ok := set[item]
	return ok
}

func deleteBranch(branch string, force bool) error {
	cmd := exec.Command("git", "branch", "-d", branch)
	if force {
		cmd = exec.Command("git", "branch", "-D", branch)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error deleting branch %s: %s", branch, output)
	}
	info("Deleted branch %s", branch)
	return nil
}

func generateCompletionScript(shell string) {
	switch shell {
	case "bash":
		bashCompletionScript()
	case "zsh":
		zshCompletionScript()
	default:
		log.Fatalf("Unsupported shell: %s. Supported shells are 'bash' and 'zsh'.", shell)
	}
}

func generateCompletionFile(shell, outputPath string) {
	file, err := os.Create(outputPath)
	if err != nil {
		log.Fatalf("Failed to create completion file: %v", err)
	}
	defer file.Close()

	switch shell {
	case "bash":
		file.WriteString(`
# Direct ggm completion for bash
# This completion script uses direct git branch commands for reliable completion

# Helper function to get branches without the '*' marker
__ggm_get_branches() {
    git branch 2>/dev/null | sed 's/^[ *]*//'
}

# Main completion function
_ggm_complete() {
    local cur prev words cword
    _init_completion || return

    # Debug line to help troubleshoot
    # echo "ggm completion: prev=$prev, cur=$cur" >&2

    case $prev in
        delete|Delete|keep|Keep)
            # Complete with branch names
            COMPREPLY=( $(compgen -W "$(__ggm_get_branches)" -- "$cur") )
            return 0
            ;;
        *)
            # If we're at a position after keep/Keep, still complete branches
            if [[ ${words[1]} == "keep" || ${words[1]} == "Keep" ]]; then
                COMPREPLY=( $(compgen -W "$(__ggm_get_branches)" -- "$cur") )
                return 0
            fi
            
            # Default to command completion
            local commands="list keep Keep delete Delete complete generate-completion --help -h"
            COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
            return 0
            ;;
    esac
}

# Register the completion function
complete -F _ggm_complete ggm
`)
	case "zsh":
		file.WriteString(`
#compdef ggm
# Direct zsh completion for ggm that bypasses the complex compdef system

# This completion script will be sourced directly by the user's .zshrc
# and provides natural tab completion for ggm commands

# Helper function to list branches
__ggm_branches() {
    git branch 2>/dev/null | sed 's/^[ *]//'
}

# Simple completion function for ggm
_ggm() {
    local -a commands
    commands=(
        "list:List all Git branches" 
        "keep:Keep only specified branches"
        "Keep:Force keep only specified branches" 
        "delete:Delete branches matching pattern"
        "Delete:Force delete branches matching pattern"
        "complete:Generate shell completion script"
        "generate-completion:Create a completion script file"
    )

    # Complete the main command
    if [[ $CURRENT -eq 2 ]]; then
        _describe "ggm command" commands
        return
    fi

    # Second-level completion based on command
    if [[ $CURRENT -ge 3 ]]; then
        case ${words[2]} in
            delete|Delete|keep|Keep)
                # Complete with branch names
                local -a branches
                branches=( $(git branch 2>/dev/null | sed 's/^[ *]//') )
                _describe "git branches" branches
                ;;
            complete)
                _values "shell type" bash zsh
                ;;
            generate-completion)
                if [[ $CURRENT -eq 3 ]]; then
                    _values "shell type" bash zsh
                else
                    _files
                fi
                ;;
        esac
    fi
}

# Register completion
compdef _ggm ggm
`)
	default:
		log.Fatalf("Unsupported shell: %s. Supported shells are 'bash' and 'zsh'.", shell)
	}

	// Add a simple helper script for direct installation
	scriptPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".install.sh"
	installScript, err := os.Create(scriptPath)
	if err == nil {
		defer installScript.Close()
		installScript.WriteString(fmt.Sprintf(`#!/bin/sh
# Installer script for ggm completion

# Determine shell type
SHELL_TYPE=$(basename "$SHELL")

if [ "$SHELL_TYPE" = "bash" ]; then
    CONFIG_FILE="$HOME/.bashrc"
    COMPLETION_FILE="%s"
elif [ "$SHELL_TYPE" = "zsh" ]; then
    CONFIG_FILE="$HOME/.zshrc"
    COMPLETION_FILE="%s"
else
    echo "Unsupported shell: $SHELL_TYPE. Please add the completion manually."
    exit 1
fi

# Check if already installed
if grep -q "source.*%s" "$CONFIG_FILE"; then
    echo "Completion already installed in $CONFIG_FILE"
else
    echo "# ggm completion" >> "$CONFIG_FILE"
    echo "source \"%s\"" >> "$CONFIG_FILE"
    echo "Added completion to $CONFIG_FILE"
fi

echo "You need to restart your shell or run: source \"$COMPLETION_FILE\""
`, outputPath, outputPath, outputPath, outputPath))
		os.Chmod(scriptPath, 0755)
		fmt.Printf("Also created an installer script at %s\n", scriptPath)
		fmt.Printf("Run it with: sh %s\n", scriptPath)
	}

	fmt.Printf("Completion file generated at %s\n", outputPath)
	fmt.Printf("Add the following to your shell config file:\n")
	fmt.Printf("  source %s\n", outputPath)
	fmt.Printf("\nAfter adding, restart your terminal or run: source %s\n", outputPath)
}

func bashCompletionScript() {
	fmt.Println(`
# Bash completion script for ggm
_ggm_completion() {
    local cur prev words cword
    _get_comp_words_by_ref -n : cur prev words cword

    echo "==> bash completion: prev='$prev' cur='$cur'" >&2

    case "$prev" in
        delete|Delete)
            echo "==> Getting branches for delete command" >&2
            COMPREPLY=( $(compgen -W "$(ggm complete-branches)" -- "$cur") )
            return 0
            ;;
        keep|Keep)
            echo "==> Getting branches for keep command" >&2
            COMPREPLY=( $(compgen -W "$(ggm complete-branches)" -- "$cur") )
            return 0
            ;;
        *)
            case "${words[1]}" in
                keep|Keep)
                    echo "==> Getting branches for keep continuation" >&2
                    COMPREPLY=( $(compgen -W "$(ggm complete-branches)" -- "$cur") )
                    return 0
                    ;;
                *)
                    echo "==> Listing commands" >&2
                    local commands="list keep Keep delete Delete complete --help -h"
                    COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
                    return 0
                    ;;
            esac
            ;;
    esac
}

complete -F _ggm_completion ggm
`)
}

func zshCompletionScript() {
	fmt.Println(`
#compdef ggm

_ggm_completion() {
    local -a commands branches
    commands=(
        "list:List all Git branches"
        "keep:Keep only specified branches"
        "Keep:Force keep only specified branches"
        "delete:Delete branches matching pattern"
        "Delete:Force delete branches matching pattern" 
        "complete:Generate shell completion script"
    )

    _arguments '1: :->command' '*: :->argument'

    echo "==> zsh completion: state=$state, words=$words" >&2

    case $state in
        command)
            _describe -t commands "ggm commands" commands
            ;;
        argument)
            case $words[2] in
                delete|Delete|keep|Keep)
                    echo "==> Fetching branches for completion" >&2
                    # Use raw command to get branches to avoid nested completion issues
                    branches=($(ggm complete-branches))
                    echo "==> Found ${#branches} branches" >&2
                    compadd "$@" -- ${branches[@]}
                    ;;
                complete)
                    compadd "$@" bash zsh
                    ;;
            esac
            ;;
    esac
}

compdef _ggm_completion ggm
`)
}

// isIndexSpec checks if the input string is an index specification (number, comma-separated numbers, or ranges)
func isIndexSpec(input string) bool {
	// Remove all digits, commas, dashes, and spaces
	cleaned := strings.Map(func(r rune) rune {
		if strings.ContainsRune("0123456789,-", r) {
			return r
		}
		return -1
	}, input)
	
	// If after cleaning we have the same length, it's an index spec
	return len(cleaned) == len(input) && len(input) > 0
}

// deleteBranchesByIndexes handles deletion by index numbers
func deleteBranchesByIndexes(indexSpec string, force bool) {
	branches, currentBranch, err := listBranches()
	if err != nil {
		log.Fatal("Error listing branches:", err)
	}
	
	// Sort branches to ensure indexes match the list command output
	sort.Strings(branches)
	
	// Parse index specifications (can be single numbers or ranges like "1-4")
	var selectedBranches []string
	specs := strings.Split(indexSpec, ",")
	
	for _, spec := range specs {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			continue
		}
		
		if strings.Contains(spec, "-") {
			// Handle range (e.g., "1-4")
			rangeParts := strings.Split(spec, "-")
			if len(rangeParts) != 2 {
				warn("Invalid range format: %s. Expected format: start-end", spec)
				continue
			}
			
			start, startErr := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			end, endErr := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			
			if startErr != nil || endErr != nil {
				warn("Invalid range: %s. Both start and end must be numbers.", spec)
				continue
			}
			
			// Adjust to 0-based indexing
			start--
			end--
			
			if start < 0 || end >= len(branches) || start > end {
				warn("Range %s out of bounds. Valid range: 1-%d", spec, len(branches))
				continue
			}
			
			for i := start; i <= end; i++ {
				selectedBranches = append(selectedBranches, branches[i])
			}
		} else {
			// Handle single index
			idx, err := strconv.Atoi(spec)
			if err != nil {
				warn("Invalid index: %s. Must be a number.", spec)
				continue
			}
			
			// Adjust to 0-based indexing
			idx--
			
			if idx < 0 || idx >= len(branches) {
				warn("Index %s out of bounds. Valid range: 1-%d", spec, len(branches))
				continue
			}
			
			selectedBranches = append(selectedBranches, branches[idx])
		}
	}
	
	if len(selectedBranches) == 0 {
		status("No valid branches selected by the provided indexes.")
		return
	}
	
	confirmAndDeleteBranches(selectedBranches, currentBranch, force)
}

func showHelp() {
	title("%s - Git Branch Manager", AppName)

	fmt.Println("A tool for managing Git branches efficiently.")
	fmt.Println("")

	status("USAGE:")
	fmt.Println("  " + AppName + " [command] [options]")

	status("COMMANDS:")

	t := color.New(color.FgGreen).PrintfFunc()
	t("  list\n")
	fmt.Println("      List all Git branches in alphabetical order")
	fmt.Println("")

	t("  keep <branch1> [branch2] ...\n")
	fmt.Println("      Keep only the specified branches and delete all others")
	fmt.Println("      Requires confirmation before deletion")
	fmt.Println("")

	t("  Keep <branch1> [branch2] ...\n")
	fmt.Println("      Same as keep, but forces deletion with -D flag")
	fmt.Println("")

	t("  delete <pattern>\n")
	fmt.Println("      Delete branches matching the specified pattern")
	fmt.Println("      Patterns can use wildcards: *test, test*, or *test*")
	fmt.Println("      Requires confirmation before deletion")
	fmt.Println("")

	t("  delete <indexes>\n")
	fmt.Println("      Delete branches by their index number as shown in list output")
	fmt.Println("      Can specify single indexes (1,3,5) or ranges (1-4)")
	fmt.Println("      Requires confirmation before deletion")
	fmt.Println("")

	t("  Delete <pattern|indexes>\n")
	fmt.Println("      Same as delete, but forces deletion with -D flag")
	fmt.Println("")

	t("  complete <shell>\n")
	fmt.Println("      Generate shell completion script (bash or zsh)")
	fmt.Println("")

	t("  generate-completion <shell> <output-file>\n")
	fmt.Println("      Generate shell completion script to a file for direct sourcing")
	fmt.Println("")

	status("OPTIONS:")
	t("  --help, -h\n")
	fmt.Println("      Show this help information")

	status("EXAMPLES:")
	e := color.New(color.FgCyan).PrintfFunc()
	e("  %s list\n", AppName)
	fmt.Println("      Lists all branches")
	fmt.Println("")

	e("  %s delete test*\n", AppName)
	fmt.Println("      Deletes all branches starting with 'test'")
	fmt.Println("")

	e("  %s delete 1,3,5-7\n", AppName)
	fmt.Println("      Deletes branches with indexes 1, 3, and 5 through 7")
	fmt.Println("")

	e("  %s keep main development\n", AppName)
	fmt.Println("      Keeps only the 'main' and 'development' branches, deleting all others")
	fmt.Println("")

	status("SHELL COMPLETION:")
	fmt.Println("  To enable branch autocompletion:")
	fmt.Println("")
	fmt.Println("  Option 1: Generate and install completion (recommended):")
	e("  %s generate-completion bash|zsh ~/.ggm-completion.sh\n", AppName)
	fmt.Println("  Then run the installer script:")
	e("  sh ~/.ggm-completion.install.sh\n")
	fmt.Println("")
	fmt.Println("  Option 2: Manual installation:")
	fmt.Println("  Add this to your shell config (~/.bashrc or ~/.zshrc):")
	e("  source ~/.ggm-completion.sh\n")
	fmt.Println("")
	fmt.Println("  After installation, restart your terminal or source your config file")
}
