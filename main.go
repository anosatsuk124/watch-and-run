package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

func parseFlags() (path string, debounce time.Duration, cmd []string) {
	file := flag.String("file", "", "file to watch (required)")
	db := flag.Duration("debounce", 0, "debounce duration")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(),
			"Usage: %s -file <path> [-debounce <dur>] -- <cmd> [args]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if *file == "" || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}
	abs, err := filepath.Abs(*file)
	if err != nil {
		log.Fatalf("abs path error: %v", err)
	}
	return abs, *db, flag.Args()
}

func initWatcher(dir string) *fsnotify.Watcher {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("watcher create error: %v", err)
	}
	if err := w.Add(dir); err != nil {
		log.Fatalf("add watch dir error: %v", err)
	}
	return w
}

func execute(cmdArgs []string) {
	c := exec.Command(cmdArgs[0], cmdArgs[1:]...)
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := c.Run(); err != nil {
		log.Printf("cmd error: %v", err)
	}
}

func watch(path string, db time.Duration, cmdArgs []string) {
	watcher := initWatcher(filepath.Dir(path))
	defer watcher.Close()

	var last time.Time
	log.Printf("watching %s\n", path)

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok || filepath.Clean(ev.Name) != path {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			now := time.Now()
			if db > 0 && now.Sub(last) < db {
				continue
			}
			last = now
			log.Printf("event: %s\n", ev)
			execute(cmdArgs)

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

func main() {
	path, db, cmdArgs := parseFlags()
	watch(path, db, cmdArgs)
}
