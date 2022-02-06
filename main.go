package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"bytes"

	"github.com/alecthomas/chroma/formatters/html"
	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting"
	meta "github.com/yuin/goldmark-meta"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldhtml "github.com/yuin/goldmark/renderer/html"
	"gopkg.in/osteele/liquid.v1"
	"gopkg.in/yaml.v2"
)

// LoadYAML reads a YAML formatted file and returns an interface
func LoadYAML(file string) map[interface{}]interface{} {
	fileData, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	data := make(map[interface{}]interface{})
	err = yaml.Unmarshal(fileData, &data)
	if err != nil {
		log.Fatal(err)
	}
	return data
}

func fileNameWithoutExtension(fileName string) string {
	return strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
}

// LoadTemplate reads an HTML file and returns the content as a string
func LoadTemplate(file string) string {
	template, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	templateStr := string(template)
	return templateStr
}

// ClearDirWithExtension removes all files in the directory with a given extension
func ClearDirWithExtension(dir string, ext string) error {
	names, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, fileInfo := range names {
		file := filepath.Join([]string{dir, fileInfo.Name()}...)
		if filepath.Ext(file) == ext {
			os.RemoveAll(file)
		}
	}
	return nil
}

type MarkdownDocument struct {
	file     string
	fileName string
	source   []byte
	html     bytes.Buffer
	metaData map[string]interface{}
}

// ParseMarkdownFile loads a markdown file, parses YAML headers, and returns a MarkdownDocument struct
func ParseMarkdownFile(markdown goldmark.Markdown, file string) MarkdownDocument {
	source, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}
	var buf bytes.Buffer
	context := parser.NewContext()
	if err := markdown.Convert(source, &buf, parser.WithContext(context)); err != nil {
		panic(err)
	}
	metaData := meta.Get(context)
	fileName := fileNameWithoutExtension(file)
	doc := MarkdownDocument{file, fileName, source, buf, metaData}
	return doc
}

func main() {
	var outputDir string
	var templatesDir string
	var docsDir string
	var configFile string
	var clean bool
	var highlightingStyle string
	var cssFile string

	flag.StringVar(&outputDir, "output", "./public", "Output directory")
	flag.StringVar(&templatesDir, "templates", "./templates", "Templates directory")
	flag.StringVar(&docsDir, "docs", "./docs", "Markdown documents directory")
	flag.StringVar(&configFile, "config", "./config.yml", "YAML configuration file")
	flag.StringVar(&highlightingStyle, "highlighting", "pygments", "Syntax highlighting style")
	flag.StringVar(&cssFile, "css", "./public/assets/css/style.css", "CSS file to minify and pass as a template variable")
	flag.BoolVar(&clean, "clean", false, "Remove HTML files in output directory before processing")
	flag.Parse()

	if clean {
		log.Println("Cleaning ", outputDir)
		err := ClearDirWithExtension(outputDir, ".html")
		if err != nil {
			log.Fatal(err)
		}
	}

	// create output directory if it doesn't exist
	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	// load config
	config := LoadYAML(configFile)

	// load layout template as a string
	layoutTemplateFile := filepath.Join(templatesDir, "layout.html")
	layoutTemplateStr := LoadTemplate(layoutTemplateFile)

	// create a markdown
	markdown := goldmark.New(
		goldmark.WithExtensions(
			meta.Meta,
			mathjax.MathJax,
			extension.Table,
			highlighting.NewHighlighting(
				highlighting.WithStyle(highlightingStyle),
				highlighting.WithFormatOptions(
					html.WithLineNumbers(false),
				),
			),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			goldhtml.WithHardWraps(),
			goldhtml.WithXHTML(),
			goldhtml.WithUnsafe(),
		),
	)

	// minify the CSS file
	cssFileData, err := ioutil.ReadFile(cssFile)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Printf("CSS file %s does not exist\n", cssFile)
	}
	mini := minify.New()
	mini.AddFunc("text/css", css.Minify)
	stylesheet, err := mini.Bytes("text/css", cssFileData)
	if err != nil {
		panic(err)
	}
	// layout params
	bindings := map[string]interface{}{
		"page":       "home",
		"config":     config,
		"stylesheet": stylesheet,
	}

	// render index.html
	engine := liquid.NewEngine()
	indexHTML, err := engine.ParseAndRenderString(layoutTemplateStr, bindings)
	if err != nil {
		log.Fatalln(err)
	}

	// write index.html
	indexFile := filepath.Join(outputDir, "index.html")
	err2 := ioutil.WriteFile(indexFile, []byte(indexHTML), os.ModePerm)
	if err2 != nil {
		log.Fatalln(err2)
	}

	// iterate over markdown files in docsDir
	names, err := ioutil.ReadDir(docsDir)
	if err != nil {
		panic(err)
	}
	for _, fileInfo := range names {
		file := filepath.Join([]string{docsDir, fileInfo.Name()}...)
		if filepath.Ext(file) == ".md" {
			// load and parse the markdown file
			doc := ParseMarkdownFile(markdown, file)
			docHTMLFile := filepath.Join(outputDir, doc.fileName+".html")
			// template variables
			bindings := map[string]interface{}{
				"page":       "article",
				"config":     config,
				"meta":       doc.metaData,
				"slug":       doc.fileName,
				"content":    doc.html.String(),
				"stylesheet": stylesheet,
			}
			// render the document
			docHTML, err := engine.ParseAndRenderString(layoutTemplateStr, bindings)
			if err != nil {
				log.Fatalln(err)
			}
			// write document HTML
			err2 := ioutil.WriteFile(docHTMLFile, []byte(docHTML), os.ModePerm)
			if err2 != nil {
				log.Fatalln(err2)
			}
		}
	}
}
