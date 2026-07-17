package main

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-isatty"
	"github.com/rivo/tview"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/pottom/cdu/cmd/cdu/app"
	"github.com/pottom/cdu/internal/config"
	"github.com/pottom/cdu/internal/theme"
	"github.com/pottom/cdu/pkg/device"
)

var (
	af        *app.Flags
	configErr error
)

var rootCmd = &cobra.Command{
	Use:   "cdu [directory_to_scan]",
	Short: "Pretty fast disk usage analyzer with a Charm interface",
	Long: `Pretty fast disk usage analyzer written in Go.

cdu is a fork of gdu (https://github.com/dundee/gdu) by Daniel Milde. The disk
analysis engine is gdu's, unchanged; what cdu adds is a new interactive interface
built on the Charm stack, themes, and recoverable deletes. The non-interactive and
JSON export modes are byte-for-byte identical to gdu's, and --classic gives you
gdu's original interface. This is not the official gdu — report cdu's own bugs at
https://github.com/pottom/cdu/issues.

Intended primarily for SSD disks where it can fully utilize parallel processing.
However HDDs work as well, but the performance gain is not so huge.
`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE:         runE,
}

// themesCmd lists the bundled color themes.
//
// Root takes a directory as its argument, so `cdu themes` is ambiguous with a
// directory actually called `themes` — a Hugo site has one, and cobra resolves
// the subcommand first. Rather than leave that as a trap, the listing notices
// and says how to scan it instead.
var themesCmd = &cobra.Command{
	Use:          "themes",
	Short:        "List the bundled color themes",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// This is the one command where a broken theme is the whole subject, and
		// there is no alternate screen here to swallow the warning.
		for _, problem := range af.ThemeProblems {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", problem)
		}
		resolved, err := theme.Resolve(&af.Theme, af.ThemeName)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %v\n", err)
		}
		// With no home directory there is nowhere to point someone at, so the
		// listing simply does not offer. It is not worth an error: the themes above
		// it are the answer they came for.
		dir, dirErr := config.ThemeDir()
		if dirErr != nil {
			dir = ""
		}
		if err := theme.List(cmd.OutOrStdout(), resolved.Name, dir); err != nil {
			return err
		}
		if info, err := os.Stat("themes"); err == nil && info.IsDir() {
			fmt.Fprintln(cmd.OutOrStdout(),
				"\nNote: ./themes is a directory. To scan it rather than list themes, run `cdu ./themes`.")
		}
		return nil
	},
}

// themesDumpCmd prints a theme's file.
//
// It is what makes the bundled themes being files mean anything to someone
// holding only the binary: they are embedded, so "copy one and edit it" would
// otherwise mean a trip to GitHub.
var themesDumpCmd = &cobra.Command{
	Use:   "dump NAME",
	Short: "Print a theme's YAML, to save as the start of your own",
	Example: "  cdu themes dump charm > ~/.config/cdu/themes/mine.yaml\n" +
		"  cdu --theme mine",
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := theme.Dump(args[0])
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	},
}

// nolint:funlen // a lot of flags to initialize
func init() {
	af = &app.Flags{Style: app.Style{ProgressModal: app.ProgressModalOpts{ShowDiskProgressBar: true}}}
	flags := rootCmd.Flags()
	flags.StringVar(&af.CfgFile, "config-file", "",
		"Read config from file (default is $XDG_CONFIG_HOME/cdu/cdu.yaml, else ~/.config/cdu/cdu.yaml; falls back to a gdu config if there is one)")
	flags.StringVarP(&af.LogFile, "log-file", "l", "/dev/null", "Path to a logfile")
	flags.StringVarP(&af.OutputFile, "output-file", "o", "", "Export all info into file as JSON")
	flags.StringVarP(&af.InputFile, "input-file", "f", "", "Import analysis from JSON file")
	flags.IntVarP(&af.MaxCores, "max-cores", "m", runtime.NumCPU(), fmt.Sprintf("Set max cores that cdu will use. %d cores available", runtime.NumCPU()))
	flags.BoolVar(&af.SequentialScanning, "sequential", false, "Use sequential scanning (intended for rotating HDDs)")
	flags.BoolVarP(&af.ShowVersion, "version", "v", false, "Print version")

	flags.StringSliceVarP(&af.TypeFilter, "type", "T", []string{}, "File types to include (e.g., --type yaml,json)")
	flags.StringSliceVarP(&af.ExcludeTypeFilter, "exclude-type", "E", []string{}, "File types to exclude (e.g., --exclude-type yaml,json)")
	flags.StringSliceVarP(&af.IgnoreDirs, "ignore-dirs", "i", []string{"/proc", "/dev", "/sys", "/run"},
		"Paths to ignore (separated by comma). Can be absolute or relative to current directory")
	flags.StringSliceVarP(&af.IgnoreDirPatterns, "ignore-dirs-pattern", "I", []string{},
		"Path patterns to ignore (separated by comma)")
	flags.StringVarP(&af.IgnoreFromFile, "ignore-from", "X", "",
		"Read path patterns to ignore from file")
	flags.BoolVarP(&af.NoHidden, "no-hidden", "H", false, "Ignore hidden directories (beginning with dot)")
	flags.BoolVarP(
		&af.FollowSymlinks, "follow-symlinks", "L", false,
		"Follow symlinks for files, i.e. show the size of the file to which symlink points to (symlinks to directories are not followed)",
	)
	flags.BoolVarP(
		&af.ShowAnnexedSize, "show-annexed-size", "A", false,
		"Use apparent size of git-annex'ed files in case files are not present locally (real usage is zero)",
	)
	flags.BoolVarP(&af.NoCross, "no-cross", "x", false, "Do not cross filesystem boundaries")
	flags.BoolVar(&af.Profiling, "enable-profiling", false, "Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/")

	flags.StringVarP(&af.DbPath, "db", "D", "", "Store analysis in database (*.sqlite for SQLite, *.badger for BadgerDB)")
	flags.BoolVarP(&af.ReadFromStorage, "read-from-storage", "r", false, "Use existing database instead of re-scanning")
	flags.BoolVar(&af.ArchiveBrowsing, "archive-browsing", false, "Enable browsing of zip/jar/tar archives (tar, tar.gz, tar.bz2, tar.xz)")
	flags.BoolVar(&af.CollapsePath, "collapse-path", false, "Collapse single-child directory chains")

	flags.BoolVarP(&af.ShowDisks, "show-disks", "d", false, "Show all mounted disks")
	flags.BoolVarP(&af.ShowApparentSize, "show-apparent-size", "a", false, "Show apparent size")
	flags.BoolVarP(&af.ShowRelativeSize, "show-relative-size", "B", false, "Show relative size")
	flags.BoolVarP(&af.NoColor, "no-color", "c", false, "Do not use colorized output")
	flags.BoolVarP(&af.ShowItemCount, "show-item-count", "C", false, "Show number of items in directory")
	flags.BoolVarP(&af.ShowMTime, "show-mtime", "M", false, "Show latest mtime of items in directory")
	// Persistent, not local: `cdu themes --theme ember` has to reach the listing
	// so it can mark which theme is in use, and a subcommand does not inherit
	// root's local flags. No backticks in the usage string — cobra reads those as
	// the flag's argument placeholder, and "see `cdu themes`" rendered the flag as
	// `--theme cdu themes`.
	rootCmd.PersistentFlags().StringVar(&af.ThemeName, "theme", "",
		"Color theme for the interactive interface (run 'cdu themes' to see them)")
	flags.BoolVar(&af.Classic, "classic", false, "Use gdu's original interface instead of the Charm one")
	flags.BoolVarP(&af.NonInteractive, "non-interactive", "n", false, "Do not run in interactive mode")
	flags.BoolVar(&af.Interactive, "interactive", false, "Force interactive mode even when output is not a TTY")
	flags.BoolVarP(&af.NoProgress, "no-progress", "p", false, "Do not show progress in non-interactive mode")
	flags.BoolVarP(&af.NoUnicode, "no-unicode", "u", false, "Do not use Unicode symbols (for size bar)")
	flags.BoolVarP(&af.Summarize, "summarize", "s", false, "Show only a total in non-interactive mode")
	flags.IntVarP(&af.Top, "top", "t", 0, "Show only top X largest files in non-interactive mode")
	flags.IntVar(&af.Depth, "depth", 0, "Show directory structure up to specified depth in non-interactive mode (0 means the flag is ignored)")
	flags.BoolVar(&af.UseSIPrefix, "si", false, "Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)")
	flags.BoolVar(&af.NoPrefix, "no-prefix", false, "Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode")
	flags.BoolVarP(&af.ShowInKiB, "show-in-kib", "k", false, "Show sizes in KiB (or kB with --si) in non-interactive mode")
	flags.BoolVar(&af.ReverseSort, "reverse-sort", false, "Reverse sorting order (smallest to largest) in non-interactive mode")
	flags.BoolVar(&af.Mouse, "mouse", false, "Use mouse")
	flags.BoolVar(&af.Icons, "icons", false,
		"Draw Nerd Font icons by file type. Needs a patched font, so it is off by default")
	flags.BoolVar(&af.NoDelete, "no-delete", false, "Do not allow deletions")
	flags.BoolVar(&af.NoViewFile, "no-view-file", false, "Do not allow viewing file contents")
	flags.BoolVar(&af.NoSpawnShell, "no-spawn-shell", false, "Do not allow spawning shell")
	flags.BoolVar(&af.WriteConfig, "write-config", false,
		"Write current configuration to ~/.config/cdu/cdu.yaml (or --config-file). This is also how you take over a gdu config")
	flags.StringVar(
		&af.Since, "since", "",
		"Include files with mtime >= WHEN. WHEN accepts RFC3339 timestamp (e.g., 2025-08-11T01:00:00-07:00) "+
			"or date only YYYY-MM-DD (calendar-day compare; includes the whole day)",
	)
	flags.StringVar(&af.Until, "until", "", "Include files with mtime <= WHEN. WHEN accepts RFC3339 timestamp or date only YYYY-MM-DD")
	flags.StringVar(&af.MaxAge, "max-age", "", "Include files with mtime no older than DURATION (e.g., 7d, 2h30m, 1y2mo)")
	flags.StringVar(&af.MinAge, "min-age", "", "Include files with mtime at least DURATION old (e.g., 30d, 1w)")

	themesCmd.AddCommand(themesDumpCmd)
	rootCmd.AddCommand(themesCmd)

	initConfig()
	initUserThemes()
	setDefaults()
}

// initUserThemes adds the themes in ~/.config/cdu/themes.
//
// A broken one is reported and skipped, never fatal: it is somebody's
// half-finished theme, not a reason to refuse to show them their disk. The
// report reaches the interactive interface on its status line, and `cdu themes`
// prints it to stderr — which is where you look when the theme you just wrote
// did not appear.
func initUserThemes() {
	dir, err := config.ThemeDir()
	if err != nil {
		return
	}
	for _, problem := range theme.LoadUserThemes(dir) {
		af.ThemeProblems = append(af.ThemeProblems, problem.Error())
	}
}

func initConfig() {
	setConfigFilePath()
	data, err := os.ReadFile(af.CfgFile)
	if err != nil {
		configErr = err
		return // config file does not exist, return
	}

	configErr = yaml.Unmarshal(data, &af)
}

func setDefaults() {
	if af.Style.Footer.BackgroundColor == "" {
		af.Style.Footer.BackgroundColor = "#2479D0"
	}
	if af.Style.Footer.TextColor == "" {
		af.Style.Footer.TextColor = "#000000"
	}
	if af.Style.Footer.NumberColor == "" {
		af.Style.Footer.NumberColor = "#FFFFFF"
	}
	if af.Style.Header.BackgroundColor == "" {
		af.Style.Header.BackgroundColor = "#2479D0"
	}
	if af.Style.Header.TextColor == "" {
		af.Style.Header.TextColor = "#000000"
	}
	if af.Style.ResultRow.NumberColor == "" {
		af.Style.ResultRow.NumberColor = "#e67100"
	}
	if af.Style.ResultRow.DirectoryColor == "" {
		af.Style.ResultRow.DirectoryColor = "#3498db"
	}
}

func setConfigFilePath() {
	command := strings.Join(os.Args, " ")
	if strings.Contains(command, "--config-file") {
		re := regexp.MustCompile("--config-file[= ]([^ ]+)")
		parts := re.FindStringSubmatch(command)

		if len(parts) > 1 {
			af.CfgFile = parts[1]
			return
		}
	}
	setDefaultConfigFilePath()
}

func setDefaultConfigFilePath() {
	path, notice, err := config.Resolve()
	if err != nil {
		configErr = err
		return
	}
	af.CfgFile = path
	af.ConfigNotice = notice
}

// writeConfigPath is where --write-config writes, which is not necessarily where
// the config was read from.
//
// When cdu falls back to a gdu config, reading it is the point and overwriting
// it is not: --write-config is how you take your own copy, which is exactly what
// the fallback notice promises. An explicit --config-file still wins, because
// then the user named the file themselves.
func writeConfigPath(command *cobra.Command) (string, error) {
	if command.Flags().Changed("config-file") && af.CfgFile != "" {
		return af.CfgFile, nil
	}
	return config.Path()
}

func runE(command *cobra.Command, args []string) error {
	var (
		termApp *tview.Application
		screen  tcell.Screen
		err     error
	)

	if af.WriteConfig {
		// --write-config dumps the config actually in effect, so the theme block
		// names the theme in use rather than being an empty map. Only the preset
		// name: writing the twelve resolved tokens would pin today's palette into
		// every config ever written, and a later cdu could never improve one.
		if resolved, terr := theme.Resolve(&af.Theme, af.ThemeName); terr == nil {
			af.Theme.Preset = resolved.Name
		}

		data, err := yaml.Marshal(af)
		if err != nil {
			return fmt.Errorf("error marshaling config file: %w", err)
		}
		path, err := writeConfigPath(command)
		if err != nil {
			return err
		}
		if err := config.WriteFile(path, data); err != nil {
			return fmt.Errorf("error writing config file %s: %w", path, err)
		}
	}

	if runtime.GOOS == "windows" && af.LogFile == "/dev/null" {
		af.LogFile = "nul"
	}

	var f *os.File
	if af.LogFile == "-" {
		f = os.Stdout
	} else {
		f, err = os.OpenFile(af.LogFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return fmt.Errorf("opening log file: %w", err)
		}
		defer func() {
			cerr := f.Close()
			if cerr != nil {
				panic(cerr)
			}
		}()
	}
	log.SetOutput(f)

	if configErr != nil {
		log.Printf("Error reading config file: %s", configErr.Error())
	}

	istty := isatty.IsTerminal(os.Stdout.Fd())

	// we are not able to analyze disk usage on Windows and Plan9
	if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
		af.ShowApparentSize = true
	}

	// Only the classic interface gets a tcell screen. The Charm interface runs its
	// own Bubble Tea loop, and a second reader attached to the same terminal would
	// race it for input — each of them swallowing every other keystroke.
	if !af.ShouldRunInNonInteractiveMode(istty) && af.Classic {
		screen, err = tcell.NewScreen()
		if err != nil {
			return fmt.Errorf("error creating screen: %w", err)
		}
		defer screen.Clear()
		defer screen.Fini()

		termApp = tview.NewApplication()
		termApp.SetScreen(screen)

		if af.Mouse {
			termApp.EnableMouse(true)
		}
	}

	a := app.App{
		Flags:       af,
		Args:        args,
		Istty:       istty,
		Writer:      os.Stdout,
		TermApp:     termApp,
		Screen:      screen,
		Getter:      device.Getter,
		PathChecker: os.Stat,
	}
	return a.Run()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
