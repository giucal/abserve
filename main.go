// Abserve implements a minimal HTTP server that serves
// a ``virtual'' resource directly from memory.
//
// 	% echo 'Hello, world!' | abserve -l :80 &
// 	[1] 1234 1235
// 	% curl localhost
// 	Hello, world!
//
// See the manual for a comprehensive description.
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

const version string = "0.0.1"

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
	path      string // The reource's (virtual) path.
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
	var printVersion bool

	flag.StringVar(&directory, "d", "",
		"serve everything else from `<directory>`")
	flag.StringVar(&fifo, "p", "",
		"ignore input and cache `<fifo>` (which must be a FIFO) on loop instead")
	flag.StringVar(&address, "l", ":8080", "listen on `<address>:<port>`")
	flag.BoolVar(&help, "h", false, "print this")
	flag.BoolVar(&printVersion, "version", false, "print version")

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

	if printVersion {
		fmt.Printf("abserve v%s\n", version)
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
