package main

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

func main() {
	args := os.Args[1:]

	if len(args) > 0 {
		switch args[0] {
		case "list":
			listSortedBranches()
		case "keep":
			if len(args) < 2 {
				fmt.Println("Usage: ggg keep [branches to keep...]")
				os.Exit(1)
			}
			keepBranches(args[1:])
		case "delete":
			if len(args) < 2 {
				fmt.Println("Usage: ggg delete [pattern]")
				os.Exit(1)
			}
			deleteBranchesByPattern(args[1])
		default:
			fmt.Println("Invalid command. Use 'list', 'keep' or 'delete'.")
			os.Exit(1)
		}
	} else {
		fmt.Println("Usage: ggg [list|keep|delete]")
		os.Exit(1)
	}
}

func confirmDeletion() bool {
	for {
		fmt.Println("Type 'yes' to confirm deletion or 'no' to cancel:")
		var input string
		fmt.Scanln(&input)
		if input == "yes" {
			return true
		} else if input == "no" {
			fmt.Println("Deletion cancelled")
			return false
		}
	}
}

func deleteBranches(branches []string) []string {
	var failed []string
	for _, branch := range branches {
		err := deleteBranch(branch)
		if err != nil {
			failed = append(failed, branch)
		}
	}
	return failed
}

func keepBranches(branchesToKeep []string) {
	allBranches, err := listBranches()
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

	confirmAndDeleteBranches(branchesToDelete)
}

func confirmAndDeleteBranches(branchesToDelete []string) bool {
	yes := confirmBranchesToDelete(branchesToDelete)
	if !yes {
		return true
	}

	_deleteBranches(branchesToDelete)
	return false
}

func deleteBranchesByPattern(pattern string) {
	branches, err := listBranches()
	if err != nil {
		fmt.Println("Error listing branches:", err)
		os.Exit(1)
	}

	isPrefixWildcard := strings.HasPrefix(pattern, "*")
	isSuffixWildcard := strings.HasSuffix(pattern, "*")
	if isPrefixWildcard || isSuffixWildcard {
		pattern = strings.Trim(pattern, "*")
	}

	var toDelete []string
	for _, branch := range branches {
		match := false
		if isPrefixWildcard && isSuffixWildcard {
			match = strings.Contains(branch, pattern)
		} else if isPrefixWildcard {
			match = strings.HasSuffix(branch, pattern)
		} else if isSuffixWildcard {
			match = strings.HasPrefix(branch, pattern)
		} else {
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

	confirmAndDeleteBranches(toDelete)
}

func _deleteBranches(toDelete []string) {
	failed := deleteBranches(toDelete)
	deletedCount := len(toDelete) - len(failed)

	if len(failed) > 0 {
		fmt.Println("Failed to delete the following branches:")
		for _, branch := range failed {
			fmt.Println(branch)
		}
	}

	fmt.Printf("%d out of %d branches were deleted.\n", deletedCount, len(toDelete))
}

func confirmBranchesToDelete(toDelete []string) bool {
	fmt.Printf("The following branches match the pattern and will be deleted:\n%s\n", strings.Join(toDelete, "\n"))

	return confirmDeletion()
}

func listSortedBranches() {
	branches, err := listBranches()
	if err != nil {
		fmt.Println("Error listing branches:", err)
		os.Exit(1)
	}

	sort.Strings(branches)
	for _, branch := range branches {
		fmt.Println(branch)
	}
}

func listBranches() ([]string, error) {
	cmd := exec.Command("git", "branch")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	branches := strings.Split(string(output), "\n")
	for i, branch := range branches {
		branch = strings.TrimSpace(branch)
		if strings.HasPrefix(branch, "*") {
			branch = strings.TrimSpace(branch[1:])
		}
		branches[i] = branch
	}

	return branches, nil
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}

func deleteBranch(branch string) error {
	cmd := exec.Command("git", "branch", "-d", branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Error deleting branch %s: %s", branch, output)
	}
	fmt.Println("Deleted branch", branch)
	return nil
}
