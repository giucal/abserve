PREFIX ?= /usr/local

all:

# Install as a typical command.
install: $(PREFIX)/bin/abserve $(PREFIX)/share/man/man1/abserve.1

uninstall:
	rm -- '$(PREFIX)/bin/abserve' \
	      '$(PREFIX)/share/man/man1/abserve.1'

clean:
	go clean
	rm -f man.1

$(PREFIX)/bin/abserve: abserve
	install $< $@

$(PREFIX)/share/man/man1/abserve.1: man.1
	cp $< $@

abserve:
	go build

man.1: man.pod abserve
	pod2man -n abserve \
	        -r '$(shell ./abserve --version)' \
	        -c 'General Commands Manual' $< > $@
