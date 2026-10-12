// Package web はフロントエンドの静的ファイルをバイナリへ同梱する。
// go:embed はソースファイルからの相対パスしか扱えないため、資産と同じディレクトリで宣言する。
package web

import "embed"

// FS は HTML / JS / CSS と Cytoscape.js を含む。
//
//go:embed index.html app.js style.css vendor/cytoscape.min.js
var FS embed.FS
