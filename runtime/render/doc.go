// Package render provides core HTML rendering primitives shared by build-time
// output, actions, fragments, and request-time page rendering.
//
// It must remain a leaf runtime dependency: this package must not import addon
// packages or own route dispatch, endpoint dispatch, SSR policy, or asset
// serving.
package render
