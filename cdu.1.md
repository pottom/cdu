---
date: {{date}}
section: 1
title: cdu
---

# NAME

cdu - Fast disk usage analyzer with a Charm interface

# SYNOPSIS

**cdu \[flags\] \[directory_to_scan\]**

**cdu themes** \| **cdu update**

# DESCRIPTION

cdu is a fast disk usage analyzer. It is a fork of gdu
(https://github.com/dundee/gdu) by Daniel Milde: the disk-analysis engine is
gdu's, unchanged, and cdu replaces the interactive interface with one built on
the Charm stack, adding themes, recoverable deletes, multi-select, and a
self-updater.

The non-interactive and JSON export modes are byte-for-byte identical to gdu's,
and **\--classic** gives you gdu's original interface. This is not the official
gdu — report cdu's own bugs at https://github.com/pottom/cdu/issues.

cdu is intended primarily for SSD disks where it can fully utilize parallel
processing. HDDs work as well, but the performance gain is not so huge.

# OPTIONS

**-h**, **\--help**\[=false\] help for cdu

**\--classic**\[=false\] Use gdu's original interface instead of the Charm one

**\--theme**=\"\" Color theme for the Charm interface (charm, midnight, ember,
phosphor, mono, or a user theme). See **cdu themes**.

**\--icons**\[=false\] Show Nerd Font file icons (needs a patched font)

**\--info**\[=true\] Show the item-info pane at the foot of the list (toggle with i)

**-i**, **\--ignore-dirs**=\[/proc,/dev,/sys,/run\]
    Paths to ignore (separated by comma).
    Supports both absolute and relative paths.

**-I**, **\--ignore-dirs-pattern**
    Path patterns to ignore (separated by comma).
    Supports both absolute and relative path patterns.

**-X**, **\--ignore-from**
    Read path patterns to ignore from file.
    Supports both absolute and relative path patterns.

**-T**, **\--type** File types to include (e.g., --type yaml,json)

**-E**, **\--exclude-type** File types to exclude (e.g., --exclude-type yaml,json)

**\--max-age** Include files with mtime no older than DURATION (e.g., 7d, 2h30m, 1y2mo)

**\--min-age** Include files with mtime at least DURATION old (e.g., 30d, 1w)

**\--since** Include files with mtime >= WHEN. WHEN accepts RFC3339 timestamp (e.g., 2025-08-11T01:00:00-07:00) or date only YYYY-MM-DD (calendar-day compare; includes the whole day)

**\--until** Include files with mtime <= WHEN. WHEN accepts RFC3339 timestamp or date only YYYY-MM-DD

**-l**, **\--log-file**=\"/dev/null\" Path to a logfile

**-m**, **\--max-cores** Set max cores that cdu will use.

**-c**, **\--no-color**\[=false\] Do not use colorized output

**-x**, **\--no-cross**\[=false\] Do not cross filesystem boundaries

**-H**, **\--no-hidden**\[=false\] Ignore hidden directories (beginning with dot)

**-L**, **\--follow-symlinks**\[=false\] Follow symlinks for files, i.e. show the
size of the file to which symlink points to (symlinks to directories are not followed)

**-n**, **\--non-interactive**\[=false\] Do not run in interactive mode

**\--interactive**\[=false\] Force interactive mode even when output is not a TTY

**-p**, **\--no-progress**\[=false\] Do not show progress in
non-interactive mode

**-u**, **\--no-unicode**\[=false\] Do not use Unicode symbols (for size bar)

**-s**, **\--summarize**\[=false\] Show only a total in non-interactive mode

**-t**, **\--top**\[=0\] Show only top X largest files in non-interactive mode

**-d**, **\--show-disks**\[=false\] Show all mounted disks

**-a**, **\--show-apparent-size**\[=false\] Show apparent size

**-C**, **\--show-item-count**\[=false\] Show number of items in directory

**-k**, **\--show-in-kib**\[=false\] Show sizes in KiB (or kB with --si) in non-interactive mode

**-M**, **\--show-mtime**\[=false\] Show latest mtime of items in directory

**\--archive-browsing**\[=false\] Enable browsing of zip/jar/tar archives (tar, tar.gz, tar.bz2, tar.xz)

**\--depth**\[=0\] Show directory structure up to specified depth in non-interactive mode (0 means the flag is ignored)

**\--collapse-path**\[=false\] Collapse single-child directory chains

**\--mouse**\[=false\] Use mouse

**\--si**\[=false\] Show sizes with decimal SI prefixes (kB, MB, GB) instead of binary prefixes (KiB, MiB, GiB)

**\--no-prefix**\[=false\] Show sizes as raw numbers without any prefixes (SI or binary) in non-interactive mode

**\--no-spawn-shell**\[=false\] Do not allow spawning shell

**\--no-delete**\[=false\] Do not allow deletions

**\--no-view-file**\[=false\] Do not allow viewing file contents

**-f**, **\--input-file** Import analysis from JSON file. If the file is \"-\", read from standard input.

**-o**, **\--output-file** Export all info into file as JSON. If the file is \"-\", write to standard output.

**\--config-file** Read config from file. Without it, cdu reads
\$XDG_CONFIG_HOME/cdu/cdu.yaml (\~/.config/cdu/cdu.yaml), falling back read-only to
an existing gdu config and saying so.

**\--write-config**\[=false\] Write the current configuration to
\~/.config/cdu/cdu.yaml (or **\--config-file**). This is also how you take over a
gdu config.

**\--enable-profiling**\[=false\] Enable collection of profiling data and provide it on http://localhost:6060/debug/pprof/

**-D**, **\--db** Store analysis in database (*.sqlite for SQLite, *.badger for BadgerDB)

**-r**, **\--read-from-storage**\[=false\] Use existing database instead of re-scanning

**-v**, **\--version**\[=false\] Print version (cdu's, and the gdu engine it embeds)

# SUBCOMMANDS

**cdu themes**

:   List the bundled color themes, each drawn in its own colors.
    **cdu themes dump NAME** prints a theme's YAML, to save as the start of your own.

**cdu update**

:   Replace the running binary with the latest release from GitHub, after verifying
    its checksum. Reads only public release assets and sends nothing. Skip it if cdu
    was installed by a package manager — update through that instead.

# KEY BINDINGS

The Charm interface (the default) is driven by:

**↑ ↓, k j** / **g G** / **pgup pgdn**

:   Move the cursor / jump to top or bottom / page.

**→ ↵ l** / **← h**

:   Enter the directory under the cursor / go to the parent (the ../ row does too;
    at the scan root, ← scans the directory above it on disk).

**/** / **f**

:   Filter this directory as you type (fuzzy, live) / find files by name across the
    whole tree (glob or substring).

**s** / **t**

:   Sort menu (then a field: size, name, count, mtime; or **d** for folders-first) /
    column menu (then **a** apparent size, **B** bars, **c** count, **m** mtime; **s**
    saves the view). **a B c m** also work directly.

**p** / **i** / **v** / **o**

:   Theme picker (previews live) / toggle the item-info pane / view a file / open a
    file in its default app.

**space** / **M** / **u**

:   Mark a row for a batch delete / open the delete queue / unmark all.

**d** / **D** / **e** / **U**

:   Trash (recoverable, does not free space) / delete permanently / empty a file /
    undo the last trash. A permission-denied delete offers to retry with sudo.

**r** / **T** / **F** / **?** / **esc** / **q**

:   Rescan / largest files / find duplicate files / help / back (cancel a scan, or
    clear marks) / quit.

# CONFIGURATION

cdu reads a YAML config from **\~/.config/cdu/cdu.yaml** (or
**\$XDG_CONFIG_HOME/cdu/cdu.yaml**), overridden by **\--config-file**. On first run,
if there is no cdu config but a gdu one exists, cdu reads the gdu config read-only and
says so; **\--write-config** takes it over into cdu's own file.

Every persistent long flag is also a config key under its flag name — for example
**show-apparent-size**, **show-item-count**, **show-mtime**, **folders-first**,
**info** (the item pane, on by default), **icons**, **mouse**, **no-hidden**, and the
**ignore-dirs** list. Two blocks have no single flag:

**sorting**

:   The default sort. **by** is one of *name*, *size*, *itemCount*, *mtime*;
    **order** is *asc* or *desc*.

**theme**

:   Colours for the Charm interface (the classic interface is unaffected). **preset**
    selects a bundled theme — *charm*, *midnight*, *ember*, *phosphor*, *mono* — and
    any token below it overrides that preset's colour for one role. Tokens are
    **\#rrggbb** only. The roles include *accent*, *text*, *selected*, *danger*,
    *size*, *dim*, and *panel*. A user theme file in **\~/.config/cdu/themes/** is
    selected by its filename with **\--theme**; see **cdu themes**.

The quickest start is **cdu \--write-config**, which writes the effective
configuration to edit. Example:

    show-apparent-size: true
    folders-first: true
    sorting:
      by: size
      order: desc
    theme:
      preset: midnight
      accent: "#ff8800"

# FILE FLAGS

Files and directories may be prefixed by a one-character
flag with following meaning:

**!**

:   An error occurred while reading this directory.

**.**

:   An error occurred while reading a subdirectory, size may be not correct.

**\@**

:  File is symlink or socket.

**H**

:  Same file was already counted (hard link).

**e**

:  Directory is empty.
