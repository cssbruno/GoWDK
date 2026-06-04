package response

// Kind identifies the response shape produced by actions, fragments, APIs, or
// full-page rendering.
type Kind string

const (
	HTML     Kind = "html"
	Redirect Kind = "redirect"
	Fragment Kind = "fragment"
	JSON     Kind = "json"
)

// Response is the generated runtime response envelope.
type Response struct {
	Kind   Kind
	Status int
	Body   string
	Target string
	URL    string
}

// HTMLBody creates a full HTML response.
func HTMLBody(status int, body string) Response {
	return Response{Kind: HTML, Status: status, Body: body}
}

// RedirectTo creates a redirect response.
func RedirectTo(url string) Response {
	return Response{Kind: Redirect, Status: 303, URL: url}
}

// FragmentFor creates a partial fragment response for a DOM target.
func FragmentFor(target, body string) Response {
	return Response{Kind: Fragment, Status: 200, Target: target, Body: body}
}
