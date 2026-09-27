package web

import "embed"

//go:embed index.html app.js styles.css mark.svg
var Assets embed.FS
