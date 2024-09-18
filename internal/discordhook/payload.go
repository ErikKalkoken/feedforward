package discordhook

// WebhookPayload represents a Discord post for a webhook.
type WebhookPayload struct {
	AllowedMentions bool    `json:"allowed_mentions,omitempty"`
	AvatarURL       string  `json:"avatar_url,omitempty"`
	Content         string  `json:"content,omitempty"`
	Embeds          []Embed `json:"embeds,omitempty"`
	Username        string  `json:"username,omitempty"`
}

// Embed represents a Discord Embed.
type Embed struct {
	Author      EmbedAuthor    `json:"author,omitempty"`
	Color       int            `json:"color,omitempty"`
	Description string         `json:"description,omitempty"`
	Fields      []EmbedField   `json:"fields,omitempty"`
	Footer      EmbedFooter    `json:"footer,omitempty"`
	Image       EmbedImage     `json:"image,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Title       string         `json:"title,omitempty"`
	Thumbnail   EmbedThumbnail `json:"thumbnail,omitempty"`
	URL         string         `json:"url,omitempty"`
}

type EmbedAuthor struct {
	Name    string `json:"name,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
	URL     string `json:"url,omitempty"`
}

type EmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type EmbedFooter struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type EmbedImage struct {
	URL string `json:"url,omitempty"`
}

type EmbedThumbnail struct {
	URL string `json:"url,omitempty"`
}
