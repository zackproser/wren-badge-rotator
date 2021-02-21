package main

import (
	"bytes"
	"errors"
	"io"

	"golang.org/x/net/html"
)

type BadgeHTML struct {
	Contents string
}

// Badge recursively searches the HTML document to find the badge node, which is wrapped in an "a" tag, or link
func Badge(doc *html.Node) (*html.Node, error) {
	var badge *html.Node
	var crawler func(*html.Node)
	crawler = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "a" {
			badge = node
			return
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			crawler(child)
		}
	}
	crawler(doc)
	if badge != nil {
		return badge, nil
	}
	return nil, errors.New("Missing <a> in the node tree")
}

// renderNode converts the supplied html node to a string
func renderNode(n *html.Node) string {
	var buf bytes.Buffer
	w := io.Writer(&buf)
	html.Render(w, n)
	return buf.String()
}

const wrapper = `
            <!doctype html>
            <html>
              <head>
                <link href="https://fonts.googleapis.com/css2?family=Roboto&display=swap" rel="stylesheet">
                <style>
                  html {
			width: 300px;
			height: 117px;
		  }

		  .wrapper-link {
                    text-decoration: none;
                  }

                  .container {
                    height: 100%;
                    padding: 12px 16px;
                    background-color: #27AE60;
                    display: flex;
                    justify-content: space-between;
                    align-items: center;
                    color: #ffffff;
                    font-family: 'Roboto';
                  }

                  .tons {
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    background-color: #ffffff;
                    color: #27AE60;
                    padding: 2px 4px;
                    border-radius: 2px;
                    width: fit-content;
                  }

                  p {
                    font-size: 12px;
                  }

                  .subject {
                    width: fit-content;
                  }

                  .header {
                    margin: 0;
                    font-size: 21px;
                    font-weight: 700;
                    max-width: 160px;
                    margin-bottom: 6px;
                  }

                  .divider {
                    min-height: 70px;
                    height: 100%;
                    border-radius: 3px;
                    width: 2px;
                    background-color: #ffffff;
                    opacity: 0.4;
                  }

                </style>
              </head>
	      <body>
	       {{ .Contents }}
	      </body>
	   </html>
`
