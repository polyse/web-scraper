package extractor

import (
	"regexp"

	"github.com/mauidude/go-readability"
)

// Clean removes html tags from string
func Clean(contentWithTags string) string {
	regexpOpenTag := regexp.MustCompile(`(<[a-zA-Z0-9]+>)`)
	content := regexpOpenTag.ReplaceAllString(contentWithTags, "")
	regexpCloseTag := regexp.MustCompile(`(</[a-zA-Z0-9]+>)`)
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
