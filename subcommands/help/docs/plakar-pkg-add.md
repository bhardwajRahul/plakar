PLAKAR-PKG-ADD(1) - General Commands Manual

# NAME

**plakar-pkg-add** - Install Plakar plugins

# SYNOPSIS

**plakar&nbsp;pkg&nbsp;add&nbsp;*plugin&nbsp;...*&zwnj;**

# DESCRIPTION

The
**plakar pkg add**
command adds a local or a remote plugin.

If
*plugin*
matches an existing local file, it is installed directly.
Otherwise, it is treated as a recipe name and downloaded from the Plakar plugin server
(requires login via plakar-login(1)).

To force local resolution, use an absolute path.
To force remote fetching, pass an HTTP or HTTPS URL.

# FILES

*~/.cache/plakar/plugins/*

> Plugin cache directory.
> Respects
> `XDG_CACHE_HOME`
> if set.

*~/.local/share/plakar/plugins*

> Plugin directory.
> Respects
> `XDG_DATA_HOME`
> if set.

# SEE ALSO

plakar-login(1),
plakar-pkg-build(1),
plakar-pkg-create(1),
plakar-pkg-rm(1),
plakar-pkg-show(1)

Plakar - November 27, 2025 - PLAKAR-PKG-ADD(1)
