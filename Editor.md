# IDE/Editor Setup

In your favorite IDE/Editor,
you should set it up so that it auto runs the following tools for you:

- `gofmt`
- `golint`
- `go vet`

## Vim/NeoVim

The easiest way to do that is to install the
[`vim-go`](https://github.com/fatih/vim-go) plugin.

## GoLand

GoLand lets you run the explicitly run all the aforementioned tools by secondary clicking (aka right clicking) anywhere in the editor area and navigating to `Go Tools` in the resulting popup menu.
Each of these menu options also have an associated keyboard shortcut, but the specific shortcut will vary depending on your platform and keymap preferences.

To automatically run these tools on every change, use the `File Watchers` feature that's common across all IntelliJ IDEA based IDEs.
To enable `File Watchers`, go to `Preferences` (`⌘,` on OSX) `->` `Tools` `->` `File Watchers`.
And click the `+` icon at the bottom of the dialog to add a watcher. Adding `go fmt`, `goimports` and `golangci-lint` will cover the aforementioned tools.

Also mark the checkbox `[x] Enable Go Modules (vgo)` under `Preferences` `->` `Go` `->` `Go Modules (vgo)`.

## VSCode

TODO
