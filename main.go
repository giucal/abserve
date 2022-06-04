// Copyright 2022 Giuseppe Calabrese.
//
// Copying and distribution of this file, with or without modification,
// are permitted in any medium without royalty provided the copyright
// notice and this notice are preserved.  This file is offered as-is,
// without any warranty.

// Command abserve implements a minimal HTTP server that serves
// an ``abstract'' resource directly from memory.
//
// 	% echo 'Hello, world!' | abserve -l :80 &
// 	% curl localhost
// 	Hello, world!
//
//
// Options and arguments
//
// Parsing conforms to the tradition of getopts.
//
// 	abserve [options] [--] [<path>]
//
// The optional argument <path> is the URL path at which the abstract resource
// must be served. If omitted, defaults to ``/''. For convenience, a starting
// ``/'' is implicit if not present.
//
// The options act as follows.
//
// 	-l, --listen <address>:<port>
//
// Binds the server to the given address and port. Default is ``:8080''.
//
// 	-d, --directory <directory>
//
// Serves any resource from <directory>, like a typical server. If
// <directory>/<path> exists, it's overridden.
//
// 	-p, --poll <file>
//
// Reads the abstract resource from <file>, which must be a FIFO (named pipe).
// After that, polls <file> repeatedly (by re-opening for reading) to fetch
// newer versions.
//
// 	-h, --help
//
// Prints a usage message to standard error.
//
//
// Behavior
//
// The server runs in the same process as abserve, until interrupted.
// The exit status is one of:
//
// 	0  on interrupt
// 	1  on runtime error
// 	2  on usage error or -h/--help
//
//
// Examples
//
// The simplest uses are:
//
// 	cmd ... | abserve  # Sink-in input from another command.
// 	abserve < file     # Read input from a file.
//
// The interesting ones involve the -p/--poll option, which lets us
// refresh the abstract resource without restarting the server. The files
// used with -p/--poll must be FIFOs:
//
// 	% mkfifo fifo      # Create a FIFO.
// 	% abserve -p fifo  # Won't serve anything until fifo is written to!
// 	^C
//
// A typical scenario is this. We have a resource to serve that can change
// over time. We launch abserve in the background in poll mode and write
// the first version of the resource to the FIFO. Then we submit updates
// by rewriting to the FIFO. For example:
//
// 	% echo 'PING LOG' > log
// 	% mkfifo fifo
// 	% abserve -l :80 -p fifo & cat log > fifo
// 	[1] 5474
// 	% for (( i = 0; i < 288; i++ )); do
// 	>     sleep 300
// 	>     echo '* * * * *' >> log
// 	>     date >> log
// 	>     ping -c5 10.0.3.1 >> log
// 	>     cat log > fifo  # Submit.
// 	> done
//
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rsc.io/getopt"
)

var prog = filepath.Base(os.Args[0])
var logger = log.New(os.Stderr, prog+": ", 0)

// Catches unrecoverable errors and logger.Fatal them.
func catch(e error) {
	if e != nil {
		logger.Fatal(e)
	}
}

// The resource's content.
var content struct {
	sync.Mutex
	bytes.Buffer
	lastMod time.Time
}

// Updates the content.
func cache(r io.Reader) {
	content.Lock()
	content.lastMod = time.Now()
	content.Truncate(0)
	_, err := content.ReadFrom(r)
	catch(err)
	content.Unlock()
}

var (
	path      string // The reource's (abstract) path.
	directory string // The directory to serve from.
	fifo      string // The FIFO to poll.
	address   string // Address and port to bind to.
)

// Serves the resource. If directory is nonempty, serves
// anything else from there.
func serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == path {
		content.Lock()
		http.ServeContent(w, r, filepath.Base(path), content.lastMod,
			bytes.NewReader(content.Bytes()))
		content.Unlock()
	} else if directory == "" {
		http.NotFound(w, r)
	} else {
		http.ServeFile(w, r, filepath.Join(directory, r.URL.Path))
	}
}

// Polls the FIFO for new content.
func recacheLoop() {
	for {
		f, err := os.Open(fifo)
		catch(err)
		cache(f)
	}
}

func synopsis() {
	fmt.Fprintf(
		os.Stderr,
		"Usage: %s [-h] [-p <fifo>] [-d <directory>] [--] [<path>]\n",
		prog)
}

// BUG(gc): Use of the flag package.
func parseArgs() {
	var help bool

	flag.StringVar(&directory, "d", "",
		"serve everything else from `<directory>`")
	flag.StringVar(&fifo, "p", "",
		"ignore input and cache `<fifo>` (which must be a FIFO) on loop instead")
	flag.StringVar(&address, "l", ":8080", "listen on `<address>:<port>`")
	flag.BoolVar(&help, "h", false, "print this")

	getopt.Alias("d", "directory")
	getopt.Alias("p", "poll")
	getopt.Alias("l", "listen")
	getopt.Alias("h", "help")

	flag.Usage = func() {
		synopsis()
		os.Exit(2)
	}

	getopt.Parse()

	if help {
		synopsis()
		fmt.Fprintf(os.Stderr, `
Cache and serve input (from memory) at http://<address>:<port>/[<path>].

Options:
`)
		getopt.PrintDefaults()
		os.Exit(2)
	}

	if flag.NArg() > 1 {
		flag.Usage()
	}

	if path = flag.Arg(0); !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
}

func main() {
	// Exit cleanly on SIGINT.
	stop := make(chan os.Signal, 1)
	go func() {
		<-stop
		os.Exit(0)
	}()
	signal.Notify(stop, os.Interrupt)

	parseArgs()

	if fifo != "" {
		f, err := os.Open(fifo)
		catch(err)

		// Check that the file is indeed a FIFO.
		stat, err := f.Stat()
		catch(err)
		if stat.Mode().Type() != fs.ModeNamedPipe {
			logger.Fatalf("not a FIFO: %s", fifo)
		}

		cache(f)
		go recacheLoop()
	} else {
		cache(os.Stdin)
	}

	http.HandleFunc("/", serve)
	logger.Fatal(http.ListenAndServe(address, nil))
}
