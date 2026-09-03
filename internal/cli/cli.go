// Package cli implements Newsfall's command-line boundary.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"newsfall/internal/app"
	"newsfall/internal/command"
	"newsfall/internal/config"
	"newsfall/internal/ui"
)

const Version = "0.1.2"

type options struct {
	ConfigPath     string
	DataPath       string
	Demo           bool
	Snapshot       bool
	Plain          bool
	Width          int
	Height         int
	Command        string
	Version        bool
	ShowConfigPath bool
}

func parse(args []string, errorOutput io.Writer) (options, error) {
	var out options
	set := flag.NewFlagSet("newsfall", flag.ContinueOnError)
	set.SetOutput(errorOutput)
	set.StringVar(&out.ConfigPath, "config", "", "configuration file (default: XDG config path)")
	set.StringVar(&out.DataPath, "data", "", "article cache file (default: XDG data path)")
	set.BoolVar(&out.Demo, "demo", false, "use the built-in offline demo feed")
	set.BoolVar(&out.Snapshot, "snapshot", false, "render one frame and exit")
	set.BoolVar(&out.Plain, "plain", false, "disable color in snapshot output")
	set.IntVar(&out.Width, "width", 0, "snapshot width in terminal cells")
	set.IntVar(&out.Height, "height", 0, "snapshot height in terminal rows")
	set.StringVar(&out.Command, "command", "", "apply one configuration command and exit")
	set.BoolVar(&out.Version, "version", false, "print version and exit")
	set.BoolVar(&out.ShowConfigPath, "config-path", false, "print the resolved configuration path")
	set.Usage = func() {}
	if err := set.Parse(args); err != nil {
		return options{}, err
	}
	if set.NArg() != 0 {
		return options{}, fmt.Errorf("unexpected arguments: %s", strings.Join(set.Args(), " "))
	}
	if out.Width < 0 || out.Height < 0 {
		return options{}, errors.New("width and height must be zero or positive")
	}
	if out.Demo && out.Command != "" {
		return options{}, errors.New("--demo and --command cannot be used together")
	}
	if out.Snapshot && out.Command != "" {
		return options{}, errors.New("--snapshot and --command cannot be used together")
	}
	return out, nil
}

// Run executes a CLI invocation and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	opts, err := parse(args, io.Discard)
	if errors.Is(err, flag.ErrHelp) {
		writeUsage(stdout)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "newsfall:", err)
		writeUsage(stderr)
		return 2
	}
	if opts.Version {
		fmt.Fprintf(stdout, "newsfall %s\n", Version)
		return 0
	}
	if opts.ShowConfigPath {
		path := opts.ConfigPath
		if path == "" {
			path = config.ConfigPath()
		}
		fmt.Fprintln(stdout, path)
		return 0
	}
	if opts.Command != "" {
		if err := runCommand(opts.ConfigPath, opts.Command, stdout); err != nil {
			fmt.Fprintln(stderr, "newsfall:", err)
			return 1
		}
		return 0
	}
	if opts.Snapshot {
		state, err := app.BuildSnapshot(opts.ConfigPath, opts.DataPath, opts.Demo, opts.Width, opts.Height)
		if err != nil {
			fmt.Fprintln(stderr, "newsfall:", err)
			return 1
		}
		frame := ui.Render(state)
		if opts.Plain {
			frame = ui.RenderPlain(state)
		}
		fmt.Fprintln(stdout, frame)
		return 0
	}
	if err := app.Run(context.Background(), app.Options{ConfigPath: opts.ConfigPath, DataPath: opts.DataPath, Demo: opts.Demo, Stdin: os.Stdin, Stdout: stdout}); err != nil {
		fmt.Fprintln(stderr, "newsfall:", err)
		return 1
	}
	return 0
}

func runCommand(path, line string, output io.Writer) error {
	if path == "" {
		path = config.ConfigPath()
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	next, effect, err := command.Execute(cfg, line)
	if err != nil {
		return err
	}
	if effect.Save {
		if err := config.Save(path, next); err != nil {
			return err
		}
	}
	if effect.Message != "" {
		fmt.Fprintln(output, effect.Message)
	}
	for _, row := range effect.Output {
		fmt.Fprintln(output, row)
	}
	if effect.Refresh {
		fmt.Fprintln(output, "live refresh will run when Newsfall is open")
	}
	return nil
}

func writeUsage(output io.Writer) {
	fmt.Fprint(output, `NEWSFALL — an ambient terminal signal desk

Usage:
  newsfall                              start the live deck
  newsfall --demo                       explore with an offline demo feed
  newsfall --demo --snapshot --plain    print a static preview
  newsfall --command 'feed list'        inspect or change configuration

Options:
  --config PATH       use a specific JSON config
  --data PATH         use a specific article cache
  --demo              use built-in offline stories
  --snapshot          render one frame and exit
  --plain             remove ANSI color from snapshots
  --width N           snapshot width (default 150)
  --height N          snapshot height (default 40)
  --command TEXT      apply an in-app command noninteractively
  --config-path       print the resolved config path
  --version           print the version
  --help              show this screen

Inside the app, press ? for keys or : for configuration commands.
`)
}
