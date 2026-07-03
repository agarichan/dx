// Package dx embeds distribution assets that live outside internal/ package
// directories (go:embed cannot reference parent paths, so the embed lives at
// the module root).
package dx

import "embed"

// RaycastExtension is the Raycast extension source, rooted at "raycast/".
// Explicit paths only — node_modules must never be embedded.
//
//go:embed raycast/src raycast/assets raycast/package.json raycast/package-lock.json raycast/tsconfig.json raycast/raycast-env.d.ts
var RaycastExtension embed.FS
