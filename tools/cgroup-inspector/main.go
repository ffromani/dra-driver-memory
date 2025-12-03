// AI-Attribution: AIA PAI Nc Hin R gemini-3.0-pro v1.0

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/ffromani/dra-driver-memory/pkg/cgroups"
)

func main() {
	rootDir := flag.String("root", cgroups.MountPoint, "Root cgroup path to inspect")
	hbSize := flag.String("size", "2MB", "Hugepage size suffix (e.g., 2MB, 1GB)")
	flag.Parse()

	// Use tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush() //nolint:errcheck

	fmt.Fprintf(w, "HIERARCHY\tRESERVED LIMIT(%s)\tLIMIT (%s)\tCURRENT\tFAILURES (Events)\n", *hbSize, *hbSize)
	fmt.Fprintf(w, "---------\t------------------\t----------\t-------\t-----------------\n")

	err := filepath.Walk(*rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		// Calculate depth for indentation
		relPath, _ := filepath.Rel(*rootDir, path)
		if relPath == "." {
			relPath = filepath.Base(*rootDir)
		}
		depth := strings.Count(relPath, string(os.PathSeparator))
		indent := strings.Repeat("  ", depth)
		nodeName := filepath.Base(path)

		// 2. Read Reserved Limit (rsvd max)
		rsvdLimitFile := filepath.Join(path, fmt.Sprintf("hugetlb.%s.rsvd.max", *hbSize))
		rsvdLimitVal := readFileValue(rsvdLimitFile)

		// 2. Read Limit (max)
		limitFile := filepath.Join(path, fmt.Sprintf("hugetlb.%s.max", *hbSize))
		limitVal := readFileValue(limitFile)

		// 2. Read Usage (current)
		currFile := filepath.Join(path, fmt.Sprintf("hugetlb.%s.current", *hbSize))
		currVal := readFileValue(currFile)

		// 3. Read Events (max hits)
		eventsFile := filepath.Join(path, fmt.Sprintf("hugetlb.%s.events", *hbSize))
		eventsVal := readEventsMax(eventsFile)

		// Print the row
		// If files don't exist (e.g. root vs leaf), values will be "-"
		fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\n", indent, nodeName, rsvdLimitVal, limitVal, currVal, eventsVal)

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking tree: %v\n", err)
	}
}

func readFileValue(path string) string {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "-"
	}
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(data))
}

func readEventsMax(path string) string {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "-"
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "max ") {
			// Return just the number "max 5" -> "5"
			return strings.TrimPrefix(line, "max ")
		}
	}
	return "0"
}
