/*
Package page validates offset pagination parameters against the policy of an
endpoint.

A client may send a page number and a size. The endpoint decides the default
size, the largest size and, optionally, the largest offset its storage serves.
[Make] combines the two into a [Page]:

	current, err := page.Make(
		page.Params{Number: number, Size: size},
		page.Config{DefaultSize: 20, MaxSize: 100},
	)
	if err != nil {
		return err
	}

	articles, err := store.List(ctx, current.Limit(), current.Offset())

[Params] holds pointers: nil means the client did not send the value and the
default applies, while a pointer to zero is an explicit invalid value. Page
numbers start at 1.

# Errors

Make returns a [*Error] with an [ErrorKind], the rejected Value and the Limit it
breaks. An offset limit is reported in page numbers, the requested page and the
last page allowed, because the page number is what the client controls.

[ErrorInvalidDefaultSize], [ErrorInvalidMaxSize] and
[ErrorDefaultSizeExceedsMaximum] describe a wrong Config. They are server bugs,
not client input, and the problem package answers them with status 500.
*/
package page
