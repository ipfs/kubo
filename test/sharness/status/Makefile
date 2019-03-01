
# Default target at the top
help:
	@echo 'TARGETS:'
	@echo ''
	@echo '  coverage     - Collects coverage info from sharness tests'
	@echo '  parse        - Parses .prove result file'
	@echo '  status       - Compute implementation status'
	@echo '  markdown     - Generate Markdown implementation status'
	@echo

coverage: results/coverage.txt

results/coverage.txt:
	@./coverage_helper.sh >$@
	@echo "results written in '$@'"

parse: results/dot_prove.txt

results/dot_prove.txt: ../.prove
	@./dot_prove_parser.sh >$@
	@echo "results written in '$@'"

status: results/status.txt

results/status.txt: results/coverage.txt results/dot_prove.txt
	@./implementation_status.sh >$@
	@echo "results written in '$@'"

markdown: results/status.md

results/status.md: results/status.txt
	@./markdown_status.sh >$@
	@echo "results written in '$@'"

.PHONY: help coverage parse status
