package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func usage() int {
	fmt.Fprintln(os.Stderr, "usage: gdpr-ropa-lint PATH [--tsv] [--fail-on medium|high|critical]\nExit: 0 ok, 1 findings at or above --fail-on, 2 bad input.")
	return 2
}

func run(args []string, stdout, stderr *os.File) int {
	var path, failOn string
	tsv := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--tsv":
			tsv = true
		case a == "--fail-on":
			i++
			if i >= len(args) || (args[i] != "medium" && args[i] != "high" && args[i] != "critical") {
				return usage()
			}
			failOn = args[i]
		case strings.HasPrefix(a, "-"):
			return usage()
		case path != "":
			return usage()
		default:
			path = a
		}
	}
	if path == "" {
		return usage()
	}
	st, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: no such file or directory: %s\n", path)
		return 2
	}
	type file struct{ label, full string }
	var files []file
	if st.IsDir() {
		base := strings.TrimRight(path, "/")
		if base == "" {
			base = "/"
		}
		ents, _ := os.ReadDir(base)
		var names []string
		for _, e := range ents {
			if !e.IsDir() && e.Type()&os.ModeSymlink == 0 && strings.HasSuffix(e.Name(), ".csv") {
				names = append(names, e.Name())
			}
		}
		sort.Strings(names)
		for _, n := range names {
			files = append(files, file{n, filepath.Join(base, n)})
		}
	} else {
		files = append(files, file{filepath.Base(path), path})
	}
	var all []Finding
	for _, f := range files {
		if fi, err := os.Stat(f.full); err != nil || fi.Size() > 20_000_000 {
			continue
		}
		b, err := os.ReadFile(f.full)
		if err != nil {
			continue
		}
		all = append(all, Audit(strings.ToValidUTF8(string(b), "�"), f.label)...)
	}
	all = Sort(all)
	head := fmt.Sprintf("%d files, %d findings", len(files), len(all))
	if tsv {
		fmt.Fprintln(stdout, "# "+head)
	} else {
		fmt.Fprintln(stdout, head)
	}
	for _, f := range all {
		if tsv {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", f.Sev, f.Rule, f.Where(), f.Msg)
		} else {
			fmt.Fprintf(stdout, "%-8s %s %s\n         %s\n", strings.ToUpper(f.Sev), f.Rule, f.Where(), f.Msg)
		}
	}
	if failOn != "" {
		for _, f := range all {
			if rank[f.Sev] <= rank[failOn] {
				return 1
			}
		}
	}
	return 0
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
