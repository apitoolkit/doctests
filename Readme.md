# Doctests: Test interactive Golang examples in your code comments

`doctest` is a tool that checks code examples and properties in Go comments.
It is similar in spirit to the [popular Python module with the same name](https://docs.python.org/3/library/doctest.html) and [Haskell library with same name](https://github.com/sol/doctest).

## Getting Started

### Installation

You can install Doctests via go modules via:

```go
go install github.com/apitoolkit/doctests@latest
```

### Setup Doctests in nvim/vim via lspconfig

Doctest includes an lsp client, which is installed via the command above. 
But to use it in your nvim editor, simply add the following code to your nvim lua config, to instruct your editor on how to use it. Note that this depends on the [lspconfig lua plugin](https://github.com/neovim/nvim-lspconfig).

```lua

local lspconfig = require 'lspconfig'
local configs = require 'lspconfig.configs'
local util = require 'lspconfig.util'

-- Check if the config is already defined (useful when reloading this file)
if not configs.doctests then
 configs.doctests = {
   default_config = {
     cmd = {'doctests', 'lsp'};
     settings = {};
     filetypes = { 'go', 'gomod', 'gotmpl' },
     root_dir = function(fname)
       return util.root_pattern 'go.work'(fname) or util.root_pattern('go.mod', '.git')(fname)
     end,
     single_file_support = true,
   };
 }
end

lspconfig.doctests.setup{}

```

### Running Doctest from CI or command line

The easiest way to run a doctest is via the CLI. To execute Doctest in the current Directory, simply run the doctest command with no arguments in the given directory. 

```go
doctests
```

Or give it a path or list of file paths or filepath glob

```go
doctests ./main.go
```
OR

```go
doctests ./main.go ./abc.go
```
OR (For every file in a project tree) 
```go
doctests ./**/*.go
```

### Wrting Doctests

`Doctest` comment lines always start with `// >>>`. The 3 greater than signs allows Doctest to detect that those lines are to be executed. The result is always inserted into the next line.

```go
package adder

// Add adds two numbers
// >>> adder.Add(1, 2)
// 3
func Add(a int, b int) int {
  return a+b
}
```
Notice that it's important to include the package name when calling functions.

## Contributors
- Anthony Alaribe
- Arne Wielding
- Mohamed Nabil 
- Omar Ahmed
