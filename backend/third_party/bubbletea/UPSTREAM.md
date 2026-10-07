# Bubble Tea dependency patch

Source: https://github.com/charmbracelet/bubbletea/tree/v2.0.10
Module: charm.land/bubbletea/v2 v2.0.10 (MIT, see LICENSE).

Only tty.go differs from the upstream Go source: initInputReader closes the
previous cancelled input reader before replacing it. Without this, every
ReleaseTerminal/RestoreTerminal cycle leaks a kqueue on macOS/BSD or an epoll
file descriptor on Linux. Rendering, input decoding and terminal modes are
unchanged.

Remove this local replacement when an upstream release includes this fix.
