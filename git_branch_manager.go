package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/fatih/color"
)

const (
	AppName = "gbm"
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
	case "complete-branches":
		// Used by shell completion to get branch names
		branches, _, err := listBranches()
		if err != nil {
			os.Exit(1)
		}
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
			log.Fatalf("Usage: %s delete|Delete [pattern]", AppName)
		}
		force := args[0] == "Delete"
		deleteBranchesByPattern(args[1], force)
		listSortedBranches()
	default:
		log.Fatalf("Invalid command. Use 'list', 'keep', 'Keep', 'delete', 'Delete', 'complete', '--help', or '-h'.")
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

func bashCompletionScript() {
	fmt.Println(`
# Bash completion script for gbm
_gbm_completion() {
    local cur prev words cword
    _get_comp_words_by_ref -n : cur prev words cword

    case "$prev" in
        delete|Delete)
            COMPREPLY=( $(compgen -W "$(gbm complete-branches)" -- "$cur") )
            return 0
            ;;
        keep|Keep)
            COMPREPLY=( $(compgen -W "$(gbm complete-branches)" -- "$cur") )
            return 0
            ;;
        *)
            case "${words[1]}" in
                keep|Keep)
                    COMPREPLY=( $(compgen -W "$(gbm complete-branches)" -- "$cur") )
                    return 0
                    ;;
                *)
                    local commands="list keep Keep delete Delete complete --help -h"
                    COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
                    return 0
                    ;;
            esac
            ;;
    esac
}

complete -F _gbm_completion gbm
`)
}

func zshCompletionScript() {
	fmt.Println(`
#compdef gbm

_gbm() {
    local state line
    typeset -A opt_args

    _arguments \
        '1: :->command' \
        '*: :->args'

    case $state in
        command)
            _values "command" \
                "list[List all Git branches]" \
                "keep[Keep only specified branches]" \
                "Keep[Force keep only specified branches]" \
                "delete[Delete branches matching pattern]" \
                "Delete[Force delete branches matching pattern]" \
                "complete[Generate shell completion script]" \
                "--help[Show help information]" \
                "-h[Show help information]"
            ;;
        args)
            case $line[1] in
                delete|Delete|keep|Keep)
                    local branches
                    branches=(${(f)"$(gbm complete-branches)"})
                    _describe -t branches "Git branches" branches
                    ;;
                complete)
                    _values "shell" "bash" "zsh"
                    ;;
            esac
            ;;
    esac
}

_gbm
`)
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

	t("  Delete <pattern>\n")
	fmt.Println("      Same as delete, but forces deletion with -D flag")
	fmt.Println("")

	t("  complete <shell>\n")
	fmt.Println("      Generate shell completion script (bash or zsh)")
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

	e("  %s keep main development\n", AppName)
	fmt.Println("      Keeps only the 'main' and 'development' branches, deleting all others")
	fmt.Println("")

	status("SHELL COMPLETION:")
	fmt.Println("  To enable branch autocompletion, add one of these lines to your shell config file:")
	fmt.Println("")
	fmt.Println("  For bash, add this to your ~/.bashrc file:")
	e("  source <(%s complete bash)\n", AppName)
	fmt.Println("")
	fmt.Println("  For zsh, add this to your ~/.zshrc file:")
	e("  source <(%s complete zsh)\n", AppName)
	fmt.Println("")
	fmt.Println("  After adding the line, restart your terminal or run 'source ~/.bashrc' or 'source ~/.zshrc'")
}
