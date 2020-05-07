package extractor

import (
	"regexp"

	"github.com/mauidude/go-readability"
	"github.com/rs/zerolog/log"
)

var (
	regexpOpenTag  = regexp.MustCompile(`(<[a-zA-Z]+>)`)
	regexpCloseTag = regexp.MustCompile(`(</[a-zA-Z]+>)`)
)

// ExtractContentFromHTML extracts article text from html.
func ExtractContentFromHTML(html string) string {
	doc, err := readability.NewDocument(html)
	if err != nil {
		log.Err(err).Msg("Can not create document from html")
	}
	doc.RemoveEmptyNodes = true
	contentWithTags := doc.Content()
	content := regexpOpenTag.ReplaceAllString(contentWithTags, "")
	content = regexpCloseTag.ReplaceAllString(content, " ")
	return content
}
