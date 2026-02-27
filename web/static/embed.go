// Package static embeds the web application's static assets (JavaScript,
// CSS) into the binary for single-binary distribution.
package static

import "embed"

// FS contains the embedded static asset files.
//
//go:embed js/*.js images/*.png
var FS embed.FS
