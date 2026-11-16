.PHONY: build test sample

build:
	mkdir -p build && go build -o build/gdpr-ropa-lint .

test: build
	go test ./... && bash tests/cli.sh

sample: build
	mkdir -p docs site
	build/gdpr-ropa-lint examples > docs/sample-output.txt; true
	build/gdpr-ropa-lint examples --tsv > docs/sample-output.tsv; true
	python3 scripts/site.py docs/sample-output.tsv site/index.html "gdpr-ropa-lint" "Lints a GDPR Article 30 record of processing activities (RoPA) kept as CSV: missing legal basis, special categories without Article 9 condition, transfers without safeguards, missing retention and unassessed profiling. Go." "Synthetic data."
