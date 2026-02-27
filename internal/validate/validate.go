package validate

import "fmt"

// Text field length limits — single source of truth for backend and frontend.
const (
	MaxTitleLength               = 500
	MaxPlaylistTitleLength       = 200
	MaxPlaylistDescriptionLength = 2000
	MaxFolderNameLength          = 100
	MaxTagNameLength             = 50
	MaxCommentBodyLength         = 5000
	MaxCompanyNameLength         = 200
	MaxFooterTextLength          = 500
	MaxCustomCSSLength           = 10 * 1024
	MaxSlackWebhookURLLength     = 500
	MaxWebhookURLLength          = 500
	MaxAPIKeyNameLength          = 100
)

func checkLen(value string, max int, field string) string {
	if len(value) > max {
		return fmt.Sprintf("%s must be %d characters or fewer", field, max)
	}
	return ""
}

func Title(s string) string              { return checkLen(s, MaxTitleLength, "title") }
func PlaylistTitle(s string) string      { return checkLen(s, MaxPlaylistTitleLength, "playlist title") }
func PlaylistDescription(s string) string {
	return checkLen(s, MaxPlaylistDescriptionLength, "playlist description")
}
func FolderName(s string) string    { return checkLen(s, MaxFolderNameLength, "folder name") }
func TagName(s string) string       { return checkLen(s, MaxTagNameLength, "tag name") }
func CommentBody(s string) string   { return checkLen(s, MaxCommentBodyLength, "comment") }
func CompanyName(s string) string   { return checkLen(s, MaxCompanyNameLength, "company name") }
func FooterText(s string) string    { return checkLen(s, MaxFooterTextLength, "footer text") }
func CustomCSS(s string) string     { return checkLen(s, MaxCustomCSSLength, "custom CSS") }
func SlackWebhookURL(s string) string {
	return checkLen(s, MaxSlackWebhookURLLength, "Slack webhook URL")
}
func WebhookURL(s string) string  { return checkLen(s, MaxWebhookURLLength, "webhook URL") }
func APIKeyName(s string) string  { return checkLen(s, MaxAPIKeyNameLength, "API key name") }

// FieldLimits returns a map of field names to max lengths for the /api/limits endpoint.
func FieldLimits() map[string]int {
	return map[string]int{
		"title":               MaxTitleLength,
		"playlistTitle":       MaxPlaylistTitleLength,
		"playlistDescription": MaxPlaylistDescriptionLength,
		"folderName":          MaxFolderNameLength,
		"tagName":             MaxTagNameLength,
		"commentBody":         MaxCommentBodyLength,
		"companyName":         MaxCompanyNameLength,
		"footerText":          MaxFooterTextLength,
		"customCSS":           MaxCustomCSSLength,
		"webhookURL":          MaxWebhookURLLength,
		"apiKeyName":          MaxAPIKeyNameLength,
	}
}
