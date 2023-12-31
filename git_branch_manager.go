package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
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

// SineWave generates a sine wave of a specified frequency and sample rate.
type SineWave struct {
	freq       float64
	sampleRate beep.SampleRate
	t          float64
}

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
		log.Fatalf("Usage: %s [list|keep|Keep|delete|Delete]", AppName)
	}

	switch args[0] {
	case "list":
		listSortedBranches()
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
	default:
		log.Fatalf("Invalid command. Use 'list', 'keep', 'Keep', 'delete' or 'Delete'.")
	}
}

func NewSineWave(freq float64, sampleRate beep.SampleRate) *SineWave {
	return &SineWave{freq: freq, sampleRate: sampleRate}
}

func (s *SineWave) Stream(samples [][2]float64) (n int, ok bool) {
	for i := range samples {
		phase := s.t * 2 * math.Pi * s.freq
		sample := float64(math.Sin(phase))
		samples[i][0] = sample
		samples[i][1] = sample
		s.t += 1.0 / float64(s.sampleRate)
	}
	return len(samples), true
}

func (s *SineWave) Err() error {
	return nil
}

func playBeepSound() {

	const sampleRate = 44100
	const freq = 440.0 // Frequency of A4
	const duration = 1 * time.Second

	sine := NewSineWave(freq, sampleRate)
	buffer := beep.NewBuffer(beep.Format{SampleRate: sampleRate, NumChannels: 2, Precision: 2})
	buffer.Append(sine)
	buffer = buffer.Slice(0, int(float64(sampleRate)*duration.Seconds()))

	speaker.Init(sampleRate, sampleRate.N(time.Second/10))
	done := make(chan bool)
	speaker.Play(beep.Seq(buffer.Streamer(0, buffer.Len()), beep.Callback(func() {
		done <- true
	})))

	<-done
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

	status("\n%d out of %d %s were deleted.\n", deletedCount, len(toDelete), toDeleteStr)
	failDeleteCount := len(toDelete) - deletedCount
	if failDeleteCount > 0 {
		warn("%d %s were not deleted due to errors.\n", failDeleteCount, deletedCountStr)
	}
}

func confirmBranchesToDelete(toDelete []string) bool {
	if len(toDelete) == 1 {
		title("The following branch matches the pattern and will be deleted:")
	} else {
		title("The following branches match the pattern and will be deleted:")
	}

	for _, branch := range toDelete {
		info(branch)
	}
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
	for i, branch := range branches {
		info("%2d. %s", i+1, branch)
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
