PLAKAR-BACKUP(1) - General Commands Manual

# NAME

**plakar-backup** - Create a new snapshot in a Kloset store

# SYNOPSIS

**plakar&nbsp;backup**
\[**-cache**&nbsp;*path*]
\[**-category**&nbsp;*category*]
\[**-check**]
\[**-dry-run**]
\[**-environment**&nbsp;*environment*]
\[**-force-timestamp**&nbsp;*timestamp*]
\[**-ignore**&nbsp;*pattern*]
\[**-ignore-file**&nbsp;*file*]
\[**-job**&nbsp;*job*]
\[**-name**&nbsp;*name*]
\[**-no-progress**]
\[**-no-xattr**]
\[**-o**&nbsp;*option*=*value*]
\[**-packfiles**&nbsp;*path*]
\[**-perimeter**&nbsp;*perimeter*]
\[**-tag**&nbsp;*tag*]
\[*place*]

# DESCRIPTION

The
**plakar backup**
command creates a new snapshot of
*place*,
or the current directory.
Snapshots can be filtered to ignore specific files or directories
based on patterns provided through options.

*place*
can be either a path, an URI, or a label with the form
"@*name*"
to reference a source connector configured with
plakar-source(1).

The options are as follows:

**-cache** *path*

> Specify a path to store the vfs cache.
> Use the special value
> 'no'
> to disable caching.
> Use the special value
> 'vfs'
> to use the in-memory vfs cache (the default).

**-category** *category*

> Set the snapshot category.

**-check**

> Perform a full check on the backup after success.

**-dry-run**

> Do not write a snapshot; instead, perform a dry run by outputting the list of
> files and directories that would be included in the backup.
> Respects all exclude patterns and other options, but makes no changes to the
> Kloset store.

**-environment** *environment*

> Set the snapshot environment.

**-force-timestamp** *timestamp*

> Specify a fixed timestamp (in ISO 8601 or relative human format) to use
> for the snapshot.
> Could be used to reimport an existing backup with the same timestamp.

**-ignore** *pattern*

> Specify individual gitignore exclusion patterns to ignore files or
> directories in the backup.
> This option can be repeated.

**-ignore-file** *file*

> Specify a file containing gitignore exclusion patterns, one per line, to
> ignore files or directories in the backup.
> This option can be repeated.

**-job** *job*

> Name the snapshot job.

**-name** *name*

> Name the snapshot.

**-no-progress**

> Do not compute or display progress.
> By default,
> **plakar backup**
> does two passes on the source of the backup: one to compute the
> number of items, and a second for processing the items themselves.
> This flag disables the pass to compute the number of items.
> It is set implicitly for some importer connectors that don't support
> the two-passes.

**-no-xattr**

> Skip extended attributes (xattrs) when creating the backup.

**-o** *option*=*value*

> Can be used to pass extra arguments to the source connector.
> The given
> *option*
> takes precedence over the configuration file.

**-packfiles** *path*

> Path where to put the temporary packfiles instead of building them in
> the default temporary directory.
> If the special value
> 'memory'
> is specified then the packfiles are built in memory.

**-perimeter** *perimeter*

> Set the snapshot perimeter.

**-tag** *tag*

> Comma-separated list of tags to apply to the snapshot.

# ENVIRONMENT

`PLAKAR_TAGS`

> Comma-separated list of tags to apply to the snapshot during backup.
> Overridden by the
> **-tag**
> command-line flag.

# EXIT STATUS

The **plakar-backup** utility exits&#160;0 on success, and&#160;&gt;0 if an error occurs.

# EXAMPLES

Create a snapshot of the current directory with two tags:

	$ plakar backup -tag daily-backup,production

Ignore files using patterns in one or more files:

	$ plakar backup -ignore-file ~/common-ignore -ignore-file ~/project-ignore /var/www

or by using patterns specified inline:

	$ plakar backup -ignore "*.tmp" -ignore "*.log" /var/www

Pass an option to the importer, in this case to don't traverse mount
points:

	$ plakar backup -o dont_traverse_fs=true /

# SEE ALSO

plakar(1),
plakar-source(1)

Plakar - May 5, 2026 - PLAKAR-BACKUP(1)
