package extractor

import (
	"testing"
)

func TestExtractor_ExtractContentFromHTML(t *testing.T) {
	htmlString := "<html><head><title>Title</title></head><body><div><p>short content</p>" +
		"<p>More content.</p>" +
		"<p>Text with link in it: <a href=\"http://www.site.ru/\" class=\"link\">some link</a></p></div>" +
		"<div id=\"banner\"></div><script>if (true) {}</script></div>" +
		"<div>And another one content block.</div>" +
		"<div><b>Some bold text.</b></div>" +
		"<img src=\"image.jpg\" class=\"image\">" +
		"<style>font-style:normal</style></body></html>"
	expected := " And another one content block. Some bold text. short content More content. " +
		"Text with link in it: some link     "
	actual := ExtractContentFromHTML(htmlString)
	if actual != expected {
		t.Errorf("\nactual:   %v \nexpected: %v", actual, expected)
	}
}
