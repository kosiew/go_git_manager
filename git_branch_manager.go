package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"
)

const AppName = "gg"

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		log.Fatalf("Usage: %s [list|keep|delete|Delete]", AppName)
	}

	switch args[0] {
	case "list":
		listSortedBranches()
	case "keep":
		if len(args) < 2 {
			log.Fatalf("Usage: %s keep [branches to keep...]", AppName)
		}
		keepBranches(args[1:])
	case "delete", "Delete":
		if len(args) < 2 {
			log.Fatalf("Usage: %s delete|Delete [pattern]", AppName)
		}
		force := args[0] == "Delete"
		deleteBranchesByPattern(args[1], force)
	default:
		log.Fatalf("Invalid command. Use 'list', 'keep' or 'delete'.")
	}
}

func confirmDeletion() bool {
	for {
		fmt.Println("\nType 'yes' to confirm deletion or 'no' to cancel:\n")
		var input string
		fmt.Scanln(&input)
		fmt.Println() // Print a newline
		if input == "yes" {
			return true
		} else if input == "no" {
			fmt.Println("Deletion cancelled")
			return false
		}
	}
}

func _deleteBranches(branches []string, force bool) map[string]string {
	failed := make(map[string]string)
	for _, branch := range branches {
		err := deleteBranch(branch, force)
		if err != nil {
			failed[branch] = err.Error()
		}
	}
	return failed
}

func keepBranches(branchesToKeep []string) {
	allBranches, currentBranch, err := listBranches()
	if err != nil {
		fmt.Println("Error listing branches:", err)
		os.Exit(1)
	}

	var branchesToDelete []string
	for _, branch := range allBranches {
		if branch != "" && !contains(branchesToKeep, branch) {
			branchesToDelete = append(branchesToDelete, branch)
		}
	}

	confirmAndDeleteBranches(branchesToDelete, currentBranch)
}

func confirmAndDeleteBranches(branchesToDelete []string, currentBranch string, force bool) bool {
	// Filter out the current branch from the branches to delete
	filteredBranches := filterCurrentBranch(branchesToDelete, currentBranch)

	if len(filteredBranches) == 0 {
		fmt.Println("No branches to delete.")
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
		fmt.Println("The current branch (" + currentBranch + ") will not be deleted.")
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
		fmt.Println("No branches match the given pattern.")
		return
	}

	confirmAndDeleteBranches(toDelete, currentBranch, force)
}

func deleteBranches(toDelete []string, force bool) {
	failed := _deleteBranches(toDelete, force)
	deletedCount := len(toDelete) - len(failed)

	if len(failed) > 0 {
		fmt.Println("\n\nFailed to delete the following branches:")
		for branch, errMsg := range failed {
			fmt.Printf("Branch: %s, Error: %s\n", branch, errMsg)
		}
	}

	fmt.Printf("\n%d out of %d branches were deleted.\n", deletedCount, len(toDelete))
}

func confirmBranchesToDelete(toDelete []string) bool {
	fmt.Printf("The following branches match the pattern and will be deleted:\n\n%s\n", strings.Join(toDelete, "\n"))

	return confirmDeletion()
}

func listSortedBranches() {
	branches, _, err := listBranches()
	if err != nil {
		fmt.Println("Error listing branches:", err)
		os.Exit(1)
	}

	sort.Strings(branches)
	for _, branch := range branches {
		fmt.Println(branch)
	}
}

func listBranches() ([]string, string, error) {
	cmd := exec.Command("git", "branch")
	output, err := cmd.Output()
	if err != nil {
		return nil, "", err
	}

	branches := strings.Split(string(output), "\n")
	var currentBranch string
	for i, branch := range branches {
		branch = strings.TrimSpace(branch)
		if strings.HasPrefix(branch, "*") {
			branch = strings.TrimSpace(branch[1:])
			currentBranch = branch
		}
		branches[i] = branch
	}

	return branches, currentBranch, nil
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
	fmt.Println("Deleted branch", branch)
	return nil
}
