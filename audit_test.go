package main

import (
	"math/rand"
	"sort"
	"strings"
	"testing"
)

const hdr = "activity,purpose,data_subjects,legal_basis,data_categories,special_categories,art9_condition,recipients,third_country,transfer_safeguard,retention,security_measures,lia_ref,profiling,dpia\n"

// base returns a clean row with overrides applied by column name.
func base(over map[string]string) string {
	cols := []string{"activity", "purpose", "data_subjects", "legal_basis", "data_categories", "special_categories", "art9_condition", "recipients", "third_country", "transfer_safeguard", "retention", "security_measures", "lia_ref", "profiling", "dpia"}
	def := map[string]string{"activity": "Billing", "purpose": "Invoice", "data_subjects": "customers", "legal_basis": "contract", "data_categories": "name", "recipients": "none", "retention": "5y", "security_measures": "MFA", "profiling": "no", "dpia": "no"}
	var vals []string
	for _, c := range cols {
		v, ok := over[c]
		if !ok {
			v = def[c]
		}
		if strings.ContainsAny(v, ",\"\n") {
			v = "\"" + strings.ReplaceAll(v, "\"", "\"\"") + "\""
		}
		vals = append(vals, v)
	}
	return strings.Join(vals, ",") + "\n"
}

func rules(text string) string {
	var r []string
	for _, f := range Audit(text, "t.csv") {
		r = append(r, f.Rule)
	}
	sort.Strings(r)
	return strings.Join(r, ",")
}

func expect(t *testing.T, name, text, want string) {
	t.Helper()
	if got := rules(text); got != want {
		t.Errorf("%s: got %q want %q", name, got, want)
	}
}

func TestClean(t *testing.T) {
	expect(t, "clean row", hdr+base(nil), "")
	expect(t, "header only", hdr, "")
}

func TestLegalBasis(t *testing.T) {
	for _, b := range []string{"consent", "contract", "legal_obligation", "vital_interests", "public_task", "CONTRACT"} {
		expect(t, b, hdr+base(map[string]string{"legal_basis": b}), "")
	}
	expect(t, "empty", hdr+base(map[string]string{"legal_basis": ""}), "GDP001")
	expect(t, "unknown", hdr+base(map[string]string{"legal_basis": "business_need"}), "GDP001")
}

func TestSpecialCategories(t *testing.T) {
	expect(t, "no condition", hdr+base(map[string]string{"special_categories": "health"}), "GDP002")
	expect(t, "with condition", hdr+base(map[string]string{"special_categories": "health", "art9_condition": "9(2)(b)"}), "")
	expect(t, "none text", hdr+base(map[string]string{"special_categories": "none"}), "")
	expect(t, "None capitalised", hdr+base(map[string]string{"special_categories": "None"}), "")
}

func TestTransfers(t *testing.T) {
	expect(t, "third country no safeguard", hdr+base(map[string]string{"third_country": "US"}), "GDP003")
	expect(t, "with safeguard", hdr+base(map[string]string{"third_country": "US", "transfer_safeguard": "SCC 2021"}), "")
	expect(t, "none", hdr+base(map[string]string{"third_country": "none"}), "")
	expect(t, "no", hdr+base(map[string]string{"third_country": "No"}), "")
	expect(t, "empty", hdr+base(map[string]string{"third_country": ""}), "")
}

func TestMissingFields(t *testing.T) {
	expect(t, "retention", hdr+base(map[string]string{"retention": ""}), "GDP004")
	expect(t, "security", hdr+base(map[string]string{"security_measures": ""}), "GDP005")
	expect(t, "recipients", hdr+base(map[string]string{"recipients": ""}), "GDP006")
	expect(t, "purpose", hdr+base(map[string]string{"purpose": ""}), "GDP007")
	expect(t, "data subjects empty", hdr+base(map[string]string{"data_subjects": ""}), "GDP011")
	expect(t, "no data_subjects column", "activity,purpose,legal_basis,data_categories,recipients,retention,security_measures,profiling,dpia\nBilling,Invoice,contract,name,none,5y,MFA,no,no\n", "GDP011")
	expect(t, "whitespace only", hdr+base(map[string]string{"retention": "   "}), "GDP004")
}

func TestLegitimateInterests(t *testing.T) {
	expect(t, "no LIA", hdr+base(map[string]string{"legal_basis": "legitimate_interests"}), "GDP008")
	expect(t, "with LIA", hdr+base(map[string]string{"legal_basis": "legitimate_interests", "lia_ref": "LIA-12"}), "")
}

func TestProfiling(t *testing.T) {
	expect(t, "profiling no dpia", hdr+base(map[string]string{"profiling": "yes"}), "GDP009")
	expect(t, "profiling with dpia", hdr+base(map[string]string{"profiling": "yes", "dpia": "yes"}), "")
	expect(t, "profiling true", hdr+base(map[string]string{"profiling": "TRUE"}), "GDP009")
	expect(t, "no profiling", hdr+base(map[string]string{"profiling": "no"}), "")
}

func TestDuplicates(t *testing.T) {
	expect(t, "duplicate", hdr+base(nil)+base(nil), "GDP010")
	expect(t, "case-insensitive duplicate", hdr+base(nil)+base(map[string]string{"activity": "BILLING"}), "GDP010")
	expect(t, "three times", hdr+base(nil)+base(nil)+base(nil), "GDP010,GDP010")
	expect(t, "different names", hdr+base(nil)+base(map[string]string{"activity": "Other"}), "")
}

func TestShapes(t *testing.T) {
	expect(t, "empty", "", "GDP000")
	expect(t, "blank lines", "\n\n", "GDP000")
	expect(t, "wrong header", "a,b\n1,2\n", "GDP000")
	if Audit("", "e")[0].Sev != "info" {
		t.Error("GDP000 should be info")
	}
	expect(t, "crlf", strings.ReplaceAll(hdr+base(nil), "\n", "\r\n"), "")
	expect(t, "quoted comma", hdr+base(map[string]string{"data_categories": "name, address"}), "")
	expect(t, "short row", hdr+"X,Y\n", "GDP001,GDP004,GDP005,GDP006,GDP011")
	expect(t, "long row", hdr+base(nil)[:len(base(nil))-1]+",extra,more\n", "")
	expect(t, "nul bytes", hdr+"a\x00b,p,customers,contract\n", "GDP004,GDP005,GDP006")
}

func TestLines(t *testing.T) {
	f := Audit(hdr+base(nil)+base(map[string]string{"activity": "B", "retention": ""}), "t.csv")
	if len(f) != 1 || f[0].Line != 3 || f[0].Where() != "t.csv:3" {
		t.Errorf("line numbers: %+v", f)
	}
	f = Audit(hdr+base(map[string]string{"purpose": "two\nlines"})+base(map[string]string{"activity": "B", "retention": ""}), "t.csv")
	if len(f) != 1 || f[0].Line != 4 {
		t.Errorf("line after quoted newline: %+v", f)
	}
	f = Audit(hdr+"\n"+base(map[string]string{"retention": ""}), "t.csv")
	if f[0].Line != 3 {
		t.Errorf("line after blank: %+v", f)
	}
}

func TestSort(t *testing.T) {
	in := []Finding{{"low", "B", "a", 1, ""}, {"high", "A", "z", 9, ""}, {"high", "A", "a", 10, ""}, {"high", "A", "a", 2, ""}}
	out := Sort(in)
	got := ""
	for _, f := range out {
		got += f.File + string(rune('0'+f.Line%10)) + " "
	}
	if got != "a2 a0 z9 a1 " {
		t.Errorf("sort: %q", got)
	}
}

func TestFuzz(t *testing.T) {
	rnd := rand.New(rand.NewSource(7))
	bits := []string{"", ",", "\"", "\n", "\r", "activity", "legal_basis", "contract", "yes", " ", "special_categories", "health", "\x00", "é", "a,b"}
	for i := 0; i < 3000; i++ {
		var sb strings.Builder
		for j, n := 0, 1+rnd.Intn(30); j < n; j++ {
			sb.WriteString(bits[rnd.Intn(len(bits))])
		}
		Audit(sb.String(), "f.csv")
	}
	valid := hdr + base(nil) + base(map[string]string{"third_country": "US"})
	for i := 0; i <= len(valid); i++ {
		Audit(valid[:i], "f.csv")
	}
}
