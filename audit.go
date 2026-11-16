package main

import (
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
)

// Finding is one result.
type Finding struct {
	Sev, Rule, File string
	Line            int
	Msg             string
}

// Where renders file or file:line.
func (f Finding) Where() string {
	if f.Line > 0 {
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	return f.File
}

var rank = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}

var bases = map[string]bool{"consent": true, "contract": true, "legal_obligation": true, "vital_interests": true, "public_task": true, "legitimate_interests": true}

func yes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "true", "1", "y":
		return true
	}
	return false
}

// Audit checks one RoPA CSV.
func Audit(text, label string) []Finding {
	r := csv.NewReader(strings.NewReader(strings.ReplaceAll(text, "\x00", "")))
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var rows [][]string
	var lines []int
	for {
		rec, err := r.Read()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			if _, ok := err.(*csv.ParseError); ok {
				continue
			}
			break
		}
		pos := 0
		if len(rec) > 0 {
			pos, _ = r.FieldPos(0)
		}
		blank := true
		for _, c := range rec {
			if strings.TrimSpace(c) != "" {
				blank = false
			}
		}
		if blank {
			continue
		}
		rows = append(rows, rec)
		lines = append(lines, pos)
	}
	if len(rows) == 0 {
		return []Finding{{"info", "GDP000", label, 0, "empty file, nothing to check"}}
	}
	idx := map[string]int{}
	for i, h := range rows[0] {
		k := strings.ToLower(strings.TrimSpace(h))
		if _, ok := idx[k]; !ok {
			idx[k] = i
		}
	}
	_, a := idx["activity"]
	_, b := idx["legal_basis"]
	if !a || !b {
		return []Finding{{"info", "GDP000", label, 0, "no header with activity and legal_basis columns"}}
	}
	var out []Finding
	seen := map[string]bool{}
	for n := 1; n < len(rows); n++ {
		rec, ln := rows[n], lines[n]
		col := func(k string) string {
			i, ok := idx[k]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		name := col("activity")
		who := name
		if who == "" {
			who = "(unnamed)"
		}
		add := func(sev, rule, msg string) { out = append(out, Finding{sev, rule, label, ln, who + ": " + msg}) }
		basis := strings.ToLower(col("legal_basis"))
		if !bases[basis] {
			add("high", "GDP001", "legal basis is empty or not an Article 6 basis")
		}
		if col("special_categories") != "" && !strings.EqualFold(col("special_categories"), "none") && col("art9_condition") == "" {
			add("high", "GDP002", "special categories are processed without an Article 9 condition")
		}
		if tc := col("third_country"); tc != "" && !strings.EqualFold(tc, "none") && !strings.EqualFold(tc, "no") && col("transfer_safeguard") == "" {
			add("high", "GDP003", "data goes to "+tc+" with no transfer safeguard")
		}
		if col("retention") == "" {
			add("medium", "GDP004", "no retention period")
		}
		if col("security_measures") == "" {
			add("medium", "GDP005", "no description of security measures")
		}
		if col("recipients") == "" {
			add("medium", "GDP006", "no recipients listed (write none if there are none)")
		}
		if col("purpose") == "" {
			add("low", "GDP007", "no purpose")
		}
		if basis == "legitimate_interests" && col("lia_ref") == "" {
			add("medium", "GDP008", "legitimate interests relied on with no legitimate interests assessment reference")
		}
		if yes(col("profiling")) && !yes(col("dpia")) {
			add("medium", "GDP009", "profiling without a completed DPIA")
		}
		if name != "" {
			key := strings.ToLower(name)
			if seen[key] {
				add("low", "GDP010", "the activity name appears more than once")
			}
			seen[key] = true
		}
	}
	return out
}

// Sort orders findings by severity, file, line, rule.
func Sort(f []Finding) []Finding {
	out := append([]Finding(nil), f...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if rank[a.Sev] != rank[b.Sev] {
			return rank[a.Sev] < rank[b.Sev]
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Rule < b.Rule
	})
	return out
}
