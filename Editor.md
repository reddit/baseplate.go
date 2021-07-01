# IDE/Editor Setup

In your favorite IDE/Editor,
you should set it up so that it auto runs the following tools for you:

- `go vet`
- `gofmt -s`
- `golint` (via `go install golang.org/x/lint/golint@latest`)
- `staticcheck` (via `go install honnef.co/go/tools/cmd/staticcheck@latest`)

## Vim/NeoVim

The easiest way to do that is to install the
[`vim-go`](https://github.com/fatih/vim-go) plugin.

In addition to `vim-go`,
It's recommended to use [`ale`](https://github.com/dense-analysis/ale) for
`golint` and `staticcheck` linting:

```vim
let g:ale_linters = {
\ 'go': ['golint', 'staticcheck'],
\}
```

## GoLand

GoLand lets you run the explicitly run all the aforementioned tools by secondary clicking (aka right clicking) anywhere in the editor area and navigating to `Go Tools` in the resulting popup menu.
Each of these menu options also have an associated keyboard shortcut, but the specific shortcut will vary depending on your platform and keymap preferences.

To automatically run these tools on every change, use the `File Watchers` feature that's common across all IntelliJ IDEA based IDEs.
To enable `File Watchers`, go to `Preferences` (`⌘,` on OSX) `->` `Tools` `->` `File Watchers`.
And click the `+` icon at the bottom of the dialog to add a watcher. Adding `go fmt`, `goimports` and `golangci-lint` will cover the aforementioned tools.

Also mark the checkbox `[x] Enable Go Modules (vgo)` and `[x] Vendoring mode` under `Preferences` `->` `Go` `->` `Go Modules (vgo)`.

## VSCode

Visual Studio Code is a lightweight but powerful source code editor which runs on your desktop and is available for Windows, macOS and Linux. It comes with built-in support for JavaScript, TypeScript and Node.js and has a rich ecosystem of extensions for other languages (such as C++, C#, Java, Python, PHP, Go) and runtimes (such as .NET and Unity).

Using the Go extension for Visual Studio Code, you get language features like IntelliSense, code navigation, symbol search, bracket matching, snippets, and many more that will help you in Golang development.

### Build, lint, and vet
On save, the Go extension can run go build, go vet, and your choice of linting tool (golint or gometalinter) on the package of the current file.
You can control these features via the settings below:
```
go.buildOnSave
go.buildFlags
go.vetOnSave
go.vetFlags
go.lintOnSave
go.lintFlags
go.lintTool
go.testOnSave
```

### Formatting
By default, formatting is run when you save your Go file, as shown below:
```
"[go]": {
  "editor.formatOnSave": true,
}
```
You can choose among three formatting tools: gofmt, goreturns, and goimports by changing the setting `go.formatTool`.
```
"go.formatTool": "goimports",
```

### `staticcheck`

VSCode supports `staticcheck` through `gopls`,
Please follow
[official `gopls` doc](https://github.com/golang/tools/blob/master/gopls/doc/settings.md#staticcheck-bool)
on how to enable it in `gopls` json config inside VSCode.

### Go Language Server

The Go extension uses a host of Go tools to provide the various language features. An alternative is to use a single language server that provides the same features using the Language Server Protocol.

> Get latest stable `gopls`
```
go get golang.org/x/tools/gopls@latest
```

> Settings to control the use of the Go language server - `gopls`
Below are the settings you can use to control the use of the language server. You need to reload the VS Code window for any changes in these settings to take effect.

- Set `go.useLanguageServer` to `true` to enable the use of language server
- Use the setting `go.languageServerExperimentalFeatures` to control which features do you want to be powered by the language server. Below are the various features you can control. By default, all are set to true.
```
"go.languageServerExperimentalFeatures": {
  "format": true,
  "autoComplete": true,
  "rename": true,
  "goToDefinition": true,
  "hover": true,
  "signatureHelp": true,
  "goToTypeDefinition": true,
  "goToImplementation": true,
  "documentSymbols": true,
  "workspaceSymbols": true,
  "findReferences": true,
  "diagnostics": true,
  "documentLink": true
}
```
- Settings to `gopls`
```
"gopls": {
  "usePlaceholders": true, // add parameter placeholders when completing a function

  // Experimental settings
  "completeUnimported": true, // autocomplete unimported packages
  "watchFileChanges": true,  // watch file changes outside of the editor
  "deepCompletion": true,     // enable deep completion
}
```

## EditorConfig

We also utilize [EditorConfig](https://editorconfig.org/) and have
[our config file](.editorconfig) committed into this git repository.
Please install an [EditorConfig plugin](https://editorconfig.org/#download) to
your favorite editor/IDE.
