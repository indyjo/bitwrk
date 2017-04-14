version ?= 0.0.0
zipfile = render_bitwrk-$(version).zip

all: $(zipfile)

.PHONY: clean
clean:
	rm -f $(zipfile)

$(zipfile): render_bitwrk/*.py
	[[ ! -e $@ ]] && zip -r $@ render_bitwrk/
