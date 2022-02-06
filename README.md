# Press - static site generator in Go

Press compiles to a binary executable (`press`) that reads a directory of markdown files and generates static minified HTML documents. Supports Markdown tables, code highlighting, and inline `$` and block `$$` quoted math. Also minifies a CSS file (default `./assets/css/style.css`). Uses Liquid template formatting.