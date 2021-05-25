.POSIX:
.SUFFIXES:

GO = go
RM = rm
SCDOC = scdoc
GOFLAGS =
PREFIX = /usr/local
BINDIR = $(PREFIX)/bin
MANDIR = $(PREFIX)/share/man
SYSCONFDIR = /etc

goflags = $(GOFLAGS) \
	-ldflags="-X 'main.configPath=$(SYSCONFDIR)/kimchi/config'"

all: kimchi kimchi.1

kimchi:
	$(GO) build $(goflags) .
kimchi.1:
	$(SCDOC) <kimchi.1.scd >kimchi.1

clean:
	$(RM) -f kimchi kimchi.1

install:
	mkdir -p $(DESTDIR)$(BINDIR)
	mkdir -p $(DESTDIR)$(MANDIR)/man1
	mkdir -p $(DESTDIR)$(SYSCONFDIR)/kimchi
	cp -f kimchi $(DESTDIR)$(BINDIR)
	cp -f kimchi.1 $(DESTDIR)$(MANDIR)/man1

.PHONY: kimchi clean install
