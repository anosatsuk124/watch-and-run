package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

type arrayFlags []string

func (f *arrayFlags) String() string {
	return ""
}

func (f *arrayFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func parseFlags() (paths []string, debounce time.Duration, cmdArgs []string) {
	var files arrayFlags
	db := flag.Duration("debounce", 0, "debounce duration")
	flag.Var(&files, "file", "file to watch (required, repeatable)")
	flag.Usage = func() {
		log.Printf("Usage: %s -file <path> [-file <path> ...] [-debounce <dur>] -- <cmd> [args]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()

	if len(files) == 0 || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}

	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			log.Fatalf("abs path error: %v", err)
		}
		paths = append(paths, abs)
	}
	return paths, *db, flag.Args()
}

func initWatcher(dirs map[string]struct{}) *fsnotify.Watcher {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("watcher create error: %v", err)
	}
	for d := range dirs {
		if err := w.Add(d); err != nil {
			log.Fatalf("add watch dir error: %v", err)
		}
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

func watch(paths []string, db time.Duration, cmdArgs []string) {
	dirs := make(map[string]struct{})
	watchMap := make(map[string]struct{})
	for _, p := range paths {
		dirs[filepath.Dir(p)] = struct{}{}
		watchMap[p] = struct{}{}
	}

	watcher := initWatcher(dirs)
	defer watcher.Close()

	last := make(map[string]time.Time)
	log.Printf("watching %v\n", paths)

	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return
			}
			p := filepath.Clean(ev.Name)
			if _, ok := watchMap[p]; !ok {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			now := time.Now()
			if db > 0 && now.Sub(last[p]) < db {
				continue
			}
			last[p] = now
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
	paths, db, cmdArgs := parseFlags()
	watch(paths, db, cmdArgs)
}
