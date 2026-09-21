package parser

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func textByID(root *html.Node, id string) (string, error) {
	node := findByID(root, id)
	if node == nil {
		return "", fmt.Errorf("%s not found", id)
	}
	text := strings.TrimSpace(textContent(node))
	if text == "" {
		return "", fmt.Errorf("%s is empty", id)
	}
	return text, nil
}

func findByID(n *html.Node, id string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode {
		for _, attr := range n.Attr {
			if attr.Key == "id" && attr.Val == id {
				return n
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return b.String()
}

func searchHtml(n *html.Node, term string, result []*html.Node) []*html.Node {
	if n.Type == html.ElementNode && n.Data == term {
		result = append(result, n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		result = searchHtml(child, term, result)
	}
	return result
}

func extractTextIter(n *html.Node) []string {
	var data []string
	stack := []*html.Node{n}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node.Type == html.TextNode {
			txt := strings.TrimSpace(node.Data)
			if txt != "" {
				data = append(data, txt)
			}
		}
		for child := node.LastChild; child != nil; child = child.PrevSibling {
			stack = append(stack, child)
		}
	}
	return data
}
