package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/afonp/microvault/internal/tools"
)

func main() {
	// global flags
	dbPath := flag.String("db", "metadata.db", "path to metadata database")
	volumes := flag.String("volumes", "http://localhost:8081", "comma-separated list of volume servers")
	replicas := flag.Int("replicas", 3, "number of replicas")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mkv [options] <command>\n")
		fmt.Fprintf(os.Stderr, "Commands: rebuild, rebalance, verify, compact\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	cmd := flag.Arg(0)

	// initialize tools ctx
	ctx := &tools.Context{
		DBPath:   *dbPath,
		Volumes:  *volumes,
		Replicas: *replicas,
	}

	var err error
	switch cmd {
	case "rebuild":
		err = tools.Rebuild(ctx)
	case "rebalance":
		err = tools.Rebalance(ctx)
	case "verify":
		err = tools.Verify(ctx)
	case "compact":
		err = tools.Compact(ctx)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
