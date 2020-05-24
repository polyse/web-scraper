package extractor

import (
	"regexp"

	"github.com/mauidude/go-readability"
)

var (
	regexpOpenTag  = regexp.MustCompile(`(<[a-zA-Z0-9]+>)`)
	regexpCloseTag = regexp.MustCompile(`(</[a-zA-Z0-9]+>)`)
)

// Clean removes html tags from string
func Clean(contentWithTags string) string {
	content := regexpOpenTag.ReplaceAllString(contentWithTags, "")
	content = regexpCloseTag.ReplaceAllString(content, " ")
	return content
}

// ExtractContentFromHTML extracts article text from html.
func ExtractContentFromHTML(html string) (string, error) {
	doc, err := readability.NewDocument(html)
	if err != nil {
		return "", err
	}
	doc.RemoveEmptyNodes = true
	return doc.Content(), nil
}
