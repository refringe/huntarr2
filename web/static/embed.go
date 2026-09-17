// Package static embeds the web application's static assets into the binary.
package static

import "embed"

// FS contains the embedded static asset files.
//
//go:embed js/*.js images/*.png
var FS embed.FS
