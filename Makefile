all: README

README:
	go doc > $@

