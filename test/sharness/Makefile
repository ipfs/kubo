SH := $(wildcard *.sh)

.DEFAULT $(SH): ALWAYS
	$(MAKE) -C ../.. test/sharness/$@

ALWAYS: 
.PHONY: ALWAYS
